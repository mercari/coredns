package clouddns

import (
	"testing"

	"github.com/mholt/caddy"
	gauth "golang.org/x/oauth2/google"
)

func TestSetupCloudDNS(t *testing.T) {
	f := func(credential *gauth.Credentials) *CloudDNSClient {
		client, err := newCloudDNSClient()
		if err != nil {
			t.Fatalf("Failed to create the mock interface: %v", err)
		}
		return client
	}

	c := caddy.NewTestController("dns", `clouddns`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns :`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
    upstream 10.0.0.1
}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
    upstream
}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
    wat
}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	// 	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
	//     aws_access_key ACCESS_KEY_ID SEKRIT_ACCESS_KEY
	//     upstream 1.2.3.4
	// }`)
	// 	if err := setup(c, f); err != nil {
	// 		t.Fatalf("Unexpected errors: %v", err)
	// 	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
    fallthrough
}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Unexpected errors: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
		credentials
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
		credentials default
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Unexpected errors: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
		credentials default credentials
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Unexpected errors: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
		credentials default credentials extra-arg
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone testproject:testzone {
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}
	c = caddy.NewTestController("dns", `clouddns testproject {
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}
}
