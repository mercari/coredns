package clouddns

import (
	"context"

	gdns "google.golang.org/api/dns/v1"
)

// Service implements the google Cloud DNS API for the scope required by the plugin
type Service interface {
	embedToIncludeNewMethods()
}

// ManagedZonesService implements the google Cloud DNS API for the scope required by the plugin
type ManagedZonesService interface {
	Get(project string, managedZone string) managedzonesgetcall

	embedToIncludeNewMethods()
}

// ManagedZonesGetCall implements the google Cloud DNS API for the scope required by the plugin
type ManagedZonesGetCall interface {
	Do() (*managedzone, error)

	embedToIncludeNewMethods()
}

// ResourceRecordSetsListCall implements the google Cloud DNS API for the scope required by the plugin
type ResourceRecordSetsListCall interface {
	List(project string, managedZoneID string) rrsetlistcall
	Do() (*gdns.ResourceRecordSetsListResponse, error)
	Pages(ctx context.Context, f func(rrsetlistresponse) error) error

	embedToIncludeNewMethods()
}

// ResourceRecordSetsListResponse implements the google Cloud DNS API for the scope required by the plugin
type ResourceRecordSetsListResponse interface {
	Pages(ctx context.Context, f func(rrsetlistresponse) error) error

	embedToIncludeNewMethods()
}
