package clouddns

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/file"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"golang.org/x/oauth2"
	gdns "google.golang.org/api/dns/v1"
	"google.golang.org/api/googleapi"
)

// CloudDNS is a plugin that returns RR from Google CloudDNS.
type CloudDNS struct {
	Next plugin.Handler
	Fall fall.F

	zoneNames []string
	client    *CloudDNSClient
	project   string
	upstream  *upstream.Upstream

	zMu   sync.RWMutex
	zones zones
}

type zone struct {
	id  string
	z   *file.Zone
	dns string
}

type Zones []string

type zones map[string][]*zone

type CloudDNSClient struct {
	resourceRecordSetsClient resourceRecordSetsClientInterface
	managedZonesClient       managedZonesServiceInterface
}

// NewCloudDNSClient is the stub for using CloudDNS API
func NewCloudDNSClient(ctx context.Context, ts oauth2.TokenSource) (*CloudDNSClient, error) {
	client := oauth2.NewClient(ctx, ts)
	dnsClient, err := gdns.New(client)
	if err != nil {
		return nil, err
	}
	gdc := &CloudDNSClient{
		resourceRecordSetsClient: resourceRecordSetsService{dnsClient.ResourceRecordSets},
		managedZonesClient:       managedZonesService{dnsClient.ManagedZones},
	}
	return gdc, nil
}

type managedZonesServiceInterface interface {
	Get(project string, managedZone string) managedZonesGetCallInterface
}

type managedZonesGetCallInterface interface {
	Do(opts ...googleapi.CallOption) (*gdns.ManagedZone, error)
}

type resourceRecordSetsService struct {
	service *gdns.ResourceRecordSetsService
}

func (r resourceRecordSetsService) List(project string, managedZone string) resourceRecordSetsListCallInterface {
	return r.service.List(project, managedZone)
}

type managedZonesService struct {
	service *gdns.ManagedZonesService
}

func (m managedZonesService) Get(project string, managedZone string) managedZonesGetCallInterface {
	return m.service.Get(project, managedZone)
}

type resourceRecordSetsClientInterface interface {
	List(project string, managedZone string) resourceRecordSetsListCallInterface
}

type resourceRecordSetsListCallInterface interface {
	Pages(ctx context.Context, f func(*gdns.ResourceRecordSetsListResponse) error) error
}

// New reads from the keys map which uses project name as its key and hosted
// zone id lists as its values, validates that each domain name/zone id pair does
// exist, and returns a new *CloudDNS. In addition to this, upstream is passed
// for doing recursive queries against CNAMEs.
// Returns error if it cannot verify any given domain name/zone id pair.
// Keys example: map[testproject:[differentzone testzone]]
func New(ctx context.Context, c *CloudDNSClient, keys map[string][]string, up *upstream.Upstream) (*CloudDNS, error) {
	zones := make(map[string][]*zone, len(keys))
	zoneNames := make([]string, 0, len(keys))
	// Generates the project using the Corefile input, we do not support multiple projects, therefore all keys map keys should be the same string.
	var proj string
	for project := range keys {
		proj = project
	}
	for _, managedZoneNames := range keys {
		for _, managedZoneName := range managedZoneNames {
			managedZone, err := c.managedZonesClient.Get(proj, managedZoneName).Do()
			if err != nil {
				log.Errorf("Failed to get the managedZone: %v", err)
				return nil, err
			}
			managedZoneDNS := managedZone.DnsName

			if _, ok := zones[managedZoneDNS]; !ok {
				zoneNames = append(zoneNames, managedZoneDNS)
			}
			zones[managedZoneDNS] = append(zones[managedZoneDNS], &zone{id: managedZoneName, dns: managedZoneDNS, z: file.NewZone(managedZoneDNS, "")})
		}
	}

	return &CloudDNS{
		client:    c,
		project:   proj,
		zoneNames: zoneNames,
		zones:     zones,
		upstream:  up,
	}, nil
}

// ServeDNS implements the plugin.Handler.ServeDNS.
func (h *CloudDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()
	zName := plugin.Zones(h.zoneNames).Matches(qname)

	if zName == "" {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	z, ok := h.zones[zName]
	if !ok || z == nil {

		return dns.RcodeServerFailure, nil
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable = true, true
	var result file.Result
	for _, managedZone := range z {
		h.zMu.RLock()
		m.Answer, m.Ns, m.Extra, result = managedZone.z.Lookup(state, qname)
		h.zMu.RUnlock()
		if len(m.Answer) != 0 {
			break
		}
	}

	if len(m.Answer) == 0 && h.Fall.Through(qname) {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	switch result {
	case file.Success:
	case file.NoData:
	case file.NameError:
		m.Rcode = dns.RcodeNameError
	case file.Delegation:
		m.Authoritative = false
	case file.ServerFailure:
		return dns.RcodeServerFailure, nil
	}

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Run executes first update, spins up an update forever-loop.
// Returns error if first update fails.
func (h *CloudDNS) Run(ctx context.Context) error {
	if err := h.updateZones(ctx); err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Infof("\n Breaking out of CloudDNS update loop: %v", ctx.Err())
				return
			case <-time.After(1 * time.Minute):
				if err := h.updateZones(ctx); err != nil && ctx.Err() == nil /* Don't log error if ctx expired. */ {
					log.Errorf("Failed to update zones: %v", err)
				}
			}
		}
	}()
	return nil
}

func (h *CloudDNS) updateZones(ctx context.Context) error {
	errc := make(chan error)
	defer close(errc)
	for zName, z := range h.zones {
		zName := zName
		z := z
		go func(zName string, z []*zone) {
			var err error
			defer func() {
				errc <- err
			}()
			for i, managedZone := range z {
				zName = managedZone.dns
				newZ := file.NewZone(zName, "")
				newZ.Upstream = *h.upstream
				err = h.client.resourceRecordSetsClient.List(h.project, managedZone.id).Pages(ctx, func(rrs *gdns.ResourceRecordSetsListResponse) error {
					if err := updateZoneFromRRS(rrs, newZ); err != nil {
						// Maybe unsupported record type. Log and carry on.
						log.Warningf("Failed to process resource record set: %v", err)
					}
					return err
				})
				if err != nil {
					err = fmt.Errorf("failed to list resource records for %v:%v from CloudDNS: %v", zName, managedZone.id, err)
					return
				}
				h.zMu.Lock()
				(*z[i]).z = newZ
				h.zMu.Unlock()
			}
		}(zName, z)
	}
	// Collect errors (if any). This will also sync on all zones updates
	// completion.
	var errs []string
	for i := 0; i < len(h.zones); i++ {
		err := <-errc
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf("errors updating zones: %v", errs)
	}
	return nil
}

func updateZoneFromRRS(rrs *gdns.ResourceRecordSetsListResponse, z *file.Zone) error {
	for _, rr := range rrs.Rrsets {
		// Assemble RFC 1035 conforming record to pass into dns scanner.
		rdata := strings.Join(rr.Rrdatas[:], ",")
		rfc1035 := fmt.Sprintf("%s %d IN %s %s", rr.Name, rr.Ttl, rr.Type, rdata)
		log.Debugf(rfc1035)
		r, err := dns.NewRR(rfc1035)
		if err != nil {
			return fmt.Errorf("failed to parse resource record: %v", err)
		}
		log.Debugf(r.String())
		z.Insert(r)
	}
	return nil
}

// Name implements plugin.Handler.Name.
func (h *CloudDNS) Name() string { return "clouddns" }
