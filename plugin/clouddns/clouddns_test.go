package clouddns

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/plugin/test"
	request "github.com/coredns/coredns/request"
	gdns "google.golang.org/api/dns/v1"
	"google.golang.org/api/googleapi"

	"github.com/miekg/dns"
)

type mockManagedZonesGetCall struct {
	project     string
	managedZone string
}

type mockManagedZonesClient struct{}

func (m *mockManagedZonesClient) Get(project string, managedZone string) managedZonesGetCallInterface {
	return &mockManagedZonesGetCall{project: project, managedZone: managedZone}
}

func (m *mockManagedZonesGetCall) Do(opts ...googleapi.CallOption) (*gdns.ManagedZone, error) {
	zone := &gdns.ManagedZone{
		DnsName: "org.",
		Name:    m.managedZone,
	}
	return zone, nil
}

type mockResourceRecordSetsListCall struct {
	project     string
	managedZone string
}

func (m *mockResourceRecordSetsListCall) Pages(ctx context.Context, f func(*gdns.ResourceRecordSetsListResponse) error) error {
	if m.managedZone == "badzone" {
		return errors.New("bad. zone is bad")
	}
	// resp := []*gdns.ResourceRecordSet{}
	resp := map[string][]*gdns.ResourceRecordSet{}
	for _, r := range []struct {
		rType, name, value, managedZoneName string
	}{
		{"A", "example.org.", "1.2.3.4", "testzone"},
		{"AAAA", "example.org.", "2001:db8:85a3::8a2e:370:7334", "testzone"},
		{"CNAME", "sample.example.org.", "example.org", "testzone"},
		{"PTR", "example.org.", "ptr.example.org.", "testzone"},
		{"SOA", "org.", "ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400", "testzone"},
		{"NS", "com.", "ns-1536.awsdns-00.co.uk.", "testzone"},
		// Unsupported type should be ignored.
		{"YOLO", "swag.", "foobar", "testzone"},
		// hosted zone with the same name, but a different id
		{"A", "other-example.org.", "3.5.7.9", "differentzone"},
		{"SOA", "org.", "ns-15.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400", "differentzone"},
	} {
		rrs, ok := resp[r.managedZoneName]
		if !ok {
			rrs = make([]*gdns.ResourceRecordSet, 0)
		}
		rrs = append(rrs, &gdns.ResourceRecordSet{Type: r.rType,
			Name:    r.name,
			Rrdatas: []string{r.value},
			Ttl:     300,
		})
		resp[r.managedZoneName] = rrs
		fmt.Printf("\n Length of resp is: %v", len(resp))
		fmt.Printf("\n Length of rrs is: %v", len(rrs))
	}
	err := f(&gdns.ResourceRecordSetsListResponse{
		Rrsets: resp[m.managedZone],
	})
	if err != nil {
		return errors.New("paging function return false")
	}
	fmt.Print(resp)
	return nil
	// return f(&gdns.ResourceRecordSetsListResponse{Rrsets: resp})
}

type mockResourceRecordSetsClient struct{}

func (m *mockResourceRecordSetsClient) List(project string, managedZone string) resourceRecordSetsListCallInterface {
	return &mockResourceRecordSetsListCall{project: project, managedZone: managedZone}
}

func newCloudDNSClient() (*CloudDNSClient, error) {
	gdc := &CloudDNSClient{
		resourceRecordSetsClient: &mockResourceRecordSetsClient{},
		managedZonesClient:       &mockManagedZonesClient{},
	}
	return gdc, nil
}

func TestCloudDNS(t *testing.T) {
	ctx := context.Background()
	s, err := newCloudDNSClient()
	project := "testproject"
	r, err := New(ctx, s, project, map[string][]string{project: []string{"badzone"}}, &upstream.Upstream{})
	if err != nil {
		t.Fatalf("Failed to create CloudDNS: %v", err)
	}
	if err = r.Run(ctx); err == nil {
		t.Fatalf("Expected errors for zone name: badzone")
	}
	r, err = New(ctx, s, project, map[string][]string{project: []string{"differentzone", "testzone"}}, &upstream.Upstream{})
	if err != nil {
		t.Fatalf("Failed to create CloudDNS: %v", err)
	}
	r.Fall = fall.Zero
	r.Fall.SetZonesFromArgs([]string{"gov."})
	r.Next = test.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		state := request.Request{W: w, Req: r}
		qname := state.Name()
		m := new(dns.Msg)
		rcode := dns.RcodeServerFailure
		if qname == "example.gov." {
			m.SetReply(r)
			rr, err := dns.NewRR("example.gov.  300 IN  A   2.4.6.8")
			if err != nil {
				t.Fatalf("Failed to create Resource Record: %v", err)
			}
			m.Answer = []dns.RR{rr}

			m.Authoritative, m.RecursionAvailable = true, true
			rcode = dns.RcodeSuccess
		}

		m.SetRcode(r, rcode)
		w.WriteMsg(m)
		return rcode, nil
	})
	err = r.Run(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize CloudDNS: %v", err)
	}
	tests := []struct {
		qname        string
		qtype        uint16
		expectedCode int
		wantAnswer   []string // ownernames for the records in the additional section.
		wantNS       []string
		expectedErr  error
	}{
		// 0. example.org A found - success.
		{
			qname:        "example.org",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"example.org.	300	IN	A	1.2.3.4"},
		},
		// 1. example.org AAAA found - success.
		{
			qname:        "example.org",
			qtype:        dns.TypeAAAA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"example.org.	300	IN	AAAA	2001:db8:85a3::8a2e:370:7334"},
		},
		// 2. exampled.org PTR found - success.
		{
			qname:        "example.org",
			qtype:        dns.TypePTR,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"example.org.	300	IN	PTR	ptr.example.org."},
		},
		// 3. sample.example.org points to example.org CNAME.
		// Query must return both CNAME and A recs.
		{
			qname:        "sample.example.org",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{
				"sample.example.org.	300	IN	CNAME	example.org.",
				"example.org.	300	IN	A	1.2.3.4",
			},
		},
		// 4. Explicit CNAME query for sample.example.org.
		// Query must return just CNAME.
		{
			qname:        "sample.example.org",
			qtype:        dns.TypeCNAME,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"sample.example.org.	300	IN	CNAME	example.org."},
		},
		// 5. Explicit SOA query for example.org.
		{
			qname:        "example.org",
			qtype:        dns.TypeSOA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"org.	300	IN	SOA	ns-15.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
		},
		// 6. Explicit SOA query for example.org.
		{
			qname:        "example.org",
			qtype:        dns.TypeNS,
			expectedCode: dns.RcodeSuccess,
			wantNS: []string{"org.	300	IN	SOA	ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
		},
		// 7. Zone not configured.
		{
			qname:        "badexample.com",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeServerFailure,
		},
		// 8. No record found. Return SOA record.
		{
			qname:        "bad.org",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeSuccess,
			wantNS: []string{"org.	300	IN	SOA	ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
		},
		// 9. No record found. Fallthrough.
		{
			qname:        "example.gov",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"example.gov.	300	IN	A	2.4.6.8"},
		},
		// 10. other-zone.example.org is stored in a different hosted zone. success
		{
			qname:        "other-example.org",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"other-example.org.	300	IN	A	3.5.7.9"},
		},
	}

	for ti, tc := range tests {
		req := new(dns.Msg)
		req.SetQuestion(dns.Fqdn(tc.qname), tc.qtype)

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		code, err := r.ServeDNS(ctx, rec, req)

		if err != tc.expectedErr {
			t.Fatalf("Test %d: Expected error %v, but got %v", ti, tc.expectedErr, err)
		}
		if code != int(tc.expectedCode) {
			t.Fatalf("Test %d: Expected status code %s, but got %s", ti, dns.RcodeToString[tc.expectedCode], dns.RcodeToString[code])
		}

		if len(tc.wantAnswer) != len(rec.Msg.Answer) {
			t.Errorf("Test %d: Unexpected number of Answers. Want: %d, got: %d", ti, len(tc.wantAnswer), len(rec.Msg.Answer))
		} else {
			for i, gotAnswer := range rec.Msg.Answer {
				if gotAnswer.String() != tc.wantAnswer[i] {
					t.Errorf("Test %d: Unexpected answer.\nWant:\n\t%s\nGot:\n\t%s", ti, tc.wantAnswer[i], gotAnswer)
				}
			}
		}

		if len(tc.wantNS) != len(rec.Msg.Ns) {
			t.Errorf("Test %d: Unexpected NS number. Want: %d, got: %d", ti, len(tc.wantNS), len(rec.Msg.Ns))
		} else {
			for i, ns := range rec.Msg.Ns {
				got, ok := ns.(*dns.SOA)
				if !ok {
					t.Errorf("Test %d: Unexpected NS type. Want: SOA, got: %v", ti, reflect.TypeOf(got))
				}
				if got.String() != tc.wantNS[i] {
					t.Errorf("Test %d: Unexpected NS. Want: %v, got: %v", ti, tc.wantNS[i], got)
				}
			}
		}
	}
}
