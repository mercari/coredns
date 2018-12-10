package clouddns

import (
	"context"

	gdns "google.golang.org/api/dns/v1"
)

// We redefine types for all the structures used in this plugin from the Google /api/dns/v1 API
type (
	service             struct{ *gdns.Service }
	managedzone         struct{ *gdns.ManagedZone }
	managedzonesservice struct{ *gdns.ManagedZonesService }
	managedzonesgetcall struct {
		*gdns.ManagedZonesGetCall
		project     string
		managedZone string
	}
	rrsetservice struct {
		*gdns.ResourceRecordSetsService
	}
	rrsetlistcall struct {
		*gdns.ResourceRecordSetsListCall
	}
	rrsetlistresponse struct {
		*gdns.ResourceRecordSetsListResponse
	}
)

func (service) embedToIncludeNewMethods()             {}
func (managedzone) embedToIncludeNewMethods()         {}
func (managedzonesservice) embedToIncludeNewMethods() {}
func (managedzonesgetcall) embedToIncludeNewMethods() {}
func (rrsetservice) embedToIncludeNewMethods()        {}
func (rrsetlistcall) embedToIncludeNewMethods()       {}
func (rrsetlistresponse) embedToIncludeNewMethods()   {}

// Get() is our custom function to allow a gapi.ManagedZonesGetCall to use our Do() function
func (mzsvc managedzonesservice) Get(project string, managedZone string) managedzonesgetcall {
	return managedzonesgetcall{mzsvc.ManagedZonesService.Get(project, managedZone), project, managedZone}
	// return managedzonesgetcall{nil, "", ""}
}

// Do() is our custom function to fake the API response returned by a gapi.ManagedZonesGetCall for testing purposes.
func (mzgc managedzonesgetcall) Do() (*managedzone, error) {
	mz, err := mzgc.ManagedZonesGetCall.Do()
	if err != nil {
		return nil, err
	}
	f := &managedzone{mz}
	return f, nil
}

// List() is our custom function to fake the API response returned by a gapi.ManagedZonesGetCall for testing purposes.
func (rrsvc rrsetservice) List(project string, managedZoneID string) rrsetlistcall {

	return rrsetlistcall{rrsvc.ResourceRecordSetsService.List(project, managedZoneID)}
}

// Do() is our custom function to fake the API response returned by a gapi.ResourceRecordSetsListCall for testing purposes.
func (rrslc rrsetlistcall) Do() (*gdns.ResourceRecordSetsListResponse, error) {
	log.Info("Inside override Do in Pages")
	rrslr, err := rrslc.ResourceRecordSetsListCall.Do()
	if err != nil {
		return nil, err
	}
	// rrslr := &rrsetlistresponse{rrslr}
	rrsResponse := map[string][]*gdns.ResourceRecordSet{}
	for _, r := range []struct {
		rType, name, value, hostedZoneID string
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
		{"SOA", "org.", "ns-15.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400", "1357986420"},
	} {
		rrs, ok := rrsResponse[r.hostedZoneID]
		if !ok {
			rrs = make([]*gdns.ResourceRecordSet, 0)
		}
		rrs = append(rrs, &gdns.ResourceRecordSet{Type: r.rType,
			Name:    r.name,
			Rrdatas: []string{r.value},
			Ttl:     300,
		})
		rrsResponse[r.hostedZoneID] = rrs
		log.Infof("Length of rrsResponse is: %v", len(rrsResponse))
		log.Infof("Length of rrs is: %v", len(rrs))

		rrslr.Rrsets = rrs
	}

	log.Infof("Length of Rrsets is: %v", len(rrslr.Rrsets))
	return rrslr, nil
}

func (rrslc rrsetlistcall) Pages(ctx context.Context, f func(*gdns.ResourceRecordSetsListResponse) error) error {
	rrslc.ResourceRecordSetsListCall.Context(ctx)
	for {
		x, err := rrslc.Do()
		log.Infof("The NextPageToken is: %v", x.NextPageToken)
		if err != nil {
			return err
		}
		if err := f(x); err != nil {
			return err
		}
		if x.NextPageToken == "" {
			return nil
		}
		rrslc.PageToken(x.NextPageToken)
	}
}

// AdaptService is an adapter to convert a Google /api/dns/v1/Service to our extended service
func AdaptService(s *gdns.Service) service {
	return service{s}
}

// AdaptManagedZonesService is an adapter to convert a Google /api/dns/v1/Service to our extended managedzonesservice
func AdaptManagedZonesService(mzsvc *gdns.ManagedZonesService) managedzonesservice {
	return managedzonesservice{mzsvc}
}

// AdaptResourceRecordSetsService is an adapter to convert a Google /api/dns/v1/Service to our extended resourcerecordsetservice
func AdaptResourceRecordSetsService(rrsvc *gdns.ResourceRecordSetsService) rrsetservice {
	return rrsetservice{rrsvc}
}

// func (rrslc rrsetlistcall) Pages(ctx context.Context, f func(rrs *gdns.ResourceRecordSetsListResponse) error) error {
// 	rrslc.ResourceRecordSetsListCall.Context(ctx)
// 	defer rrslc.PageToken(rrslc.urlParams.Get("pageToken")) // reset paging to original point
// 	log.Infof("Inside the Pages overridden function")
// 	rrsResponse := map[string][]*gdns.ResourceRecordSet{}
// 	counter := 0
// 	for _, r := range []struct {
// 		rType, name, value, hostedZoneID string
// 	}{
// 		{"A", "example.org.", "1.2.3.4", "testzone"},
// 		{"AAAA", "example.org.", "2001:db8:85a3::8a2e:370:7334", "testzone"},
// 		{"CNAME", "sample.example.org.", "example.org", "testzone"},
// 		{"PTR", "example.org.", "ptr.example.org.", "testzone"},
// 		{"SOA", "org.", "ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400", "testzone"},
// 		{"NS", "com.", "ns-1536.awsdns-00.co.uk.", "testzone"},
// 		// Unsupported type should be ignored.
// 		{"YOLO", "swag.", "foobar", "testzone"},
// 		// hosted zone with the same name, but a different id
// 		{"A", "other-example.org.", "3.5.7.9", "differentzone"},
// 		{"SOA", "org.", "ns-15.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400", "1357986420"},
// 	} {
// 		log.Infof("For loop iterating over rr struct counter is: %v", counter)
// 		counter++
// 		log.Infof("Index is %v", r)
// 		rrs, ok := rrsResponse[r.hostedZoneID]
// 		if !ok {
// 			rrs = make([]*gdns.ResourceRecordSet, 0)
// 		}
// 		rrs = append(rrs, &gdns.ResourceRecordSet{Type: r.rType,
// 			Name:    r.name,
// 			Rrdatas: []string{r.value},
// 			Ttl:     300,
// 		})
// 		rrsResponse[r.hostedZoneID] = rrs
// 		// x, _ := rrslc.Do()
// 		log.Infof("Resource record set is: %v ", rrs[0])
// 		log.Infof("Resource record response length is: %v ", len(rrsResponse))
// 	}
// 	log.Infof("Rrsets are: %v", rrsResponse)
// 	log.Infof("Rrsets are: %v", rrsResponse)
// 	return nil
// }
