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
	gdns "google.golang.org/api/dns/v1"
)

// CloudDNS is a plugin that returns RR from Google CloudDNS.
type CloudDNS struct {
	Next plugin.Handler
	Fall fall.F

	zoneNames []string
	client    gdns.Service
	upstream  *upstream.Upstream

	zMu   sync.RWMutex
	zones zones
}

type zone struct {
	id  string
	z   *file.Zone
	dns string
}

//Zones is ok
type Zones []string

type zones map[string][]*zone

// New reads from the keys map which uses domain names as its key and hosted
// zone id lists as its values, validates that each domain name/zone id pair does
// exist, and returns a new *Route53. In addition to this, upstream is passed
// for doing recursive queries against CNAMEs.
// Returns error if it cannot verify any given domain name/zone id pair.
func New(ctx context.Context, c gdns.Service, proj string, keys map[string][]string, up *upstream.Upstream) (*CloudDNS, error) {
	log.Infof("Entering New function with %v project value", proj)
	log.Infof("Current project is %v", proj)
	zones := make(map[string][]*zone, len(keys))
	zoneNames := make([]string, 0, len(keys))
	for _, managedZoneNames := range keys {
		for _, managedZoneName := range managedZoneNames {
			log.Infof("ManagedzoneName is %v", managedZoneName)
			managedZone, err := c.ManagedZones.Get(proj, managedZoneName).Do()
			if err != nil {
				return nil, err
			}
			managedZoneID := managedZone.DnsName
			if _, ok := zones[managedZoneID]; !ok {
				zoneNames = append(zoneNames, managedZoneID)
			}
			zones[managedZoneID] = append(zones[managedZoneID], &zone{id: managedZoneName, dns: managedZoneID, z: file.NewZone(managedZoneID, "")})
		}
	}

	for i, j := range zones {
		log.Infof("Zone index is", i)
		log.Infof("Zone value is", j)
		for k, l := range j {
			log.Infof("Zone subindex is", k)
			log.Infof("Zone dns is", l.dns)
			log.Infof("Zone id is", l.id)
			log.Infof("Zone file is", l.z.All())

		}

	}
	log.Infof("keys are %v", keys)
	log.Infof("keys length is %v", len(keys))
	log.Infof("Zones are %v", zones)
	log.Infof("Zones length is %v", len(zones))

	log.Infof("Zone names are %v", zoneNames)
	log.Infof("Zones name length is %v", len(zoneNames))
	return &CloudDNS{
		client:    c,
		zoneNames: zoneNames,
		zones:     zones,
		upstream:  up,
	}, nil
}

//Matches test
func (z Zones) Matches(qname string) string {
	zone := ""
	for _, zname := range z {
		log.Infof("matches request received name %v", qname)
		log.Infof("matches request comparing is %v", zname)
		if dns.IsSubDomain(zname, qname) {
			log.Infof("%v is actually a sub-part of %v", qname, zname)
			// We want the *longest* matching zone, otherwise we may end up in a parent
			if len(zname) > len(zone) {
				zone = zname

			}
		}
	}
	log.Infof("Returned zone name is %v", zone)

	return zone
}

// ServeDNS implements the plugin.Handler.ServeDNS.
func (h *CloudDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()
	log.Infof("request comparing is %v", qname)
	for _, zo := range h.zoneNames {
		log.Infof("zoneNames part is %v", zo)
	}
	zName := Zones(h.zoneNames).Matches(qname)
	log.Infof("request compare result = (%v)", zName)

	if zName == "" {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}
	z, ok := h.zones[zName]
	if !ok || z == nil {
		log.Infof("Failed to find it ! (%v)", z)

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
	log.Infof("Entering Run function")
	log.Info(h.zones)
	if err := h.updateZones(ctx); err != nil {
		return err
	}
	go func() {
		for {
			log.Infof("Entering Run function loop")
			log.Info(h.zones)
			for i, j := range h.zones {
				log.Infof("Zone index is", i)
				log.Infof("Zone value is", j)
				for k, l := range j {
					log.Infof("Zone subindex is", k)
					log.Infof("Zone dns is", l.dns)
					log.Infof("Zone id is", l.id)
					log.Infof("Zone file is", l.z.All())

				}

			}
			log.Infof("Zones are %v", h.zones)
			log.Infof("Zones length is %v", len(h.zones))

			log.Infof("Zone names are %v", h.zoneNames)
			log.Infof("Zones name length is %v", len(h.zoneNames))
			select {
			case <-ctx.Done():
				log.Infof("Breaking out of CloudDNS update loop: %v", ctx.Err())
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
		go func(zName string, z []*zone) {
			var err error
			defer func() {
				errc <- err
			}()

			for i, managedZone := range z {
				newZ := file.NewZone(managedZone.dns, "")
				newZ.Upstream = *h.upstream
				proj := "kouzoh-p-lainra"
				err = h.client.ResourceRecordSets.List(proj, managedZone.id).Pages(ctx, func(rrs *gdns.ResourceRecordSetsListResponse) error {
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
