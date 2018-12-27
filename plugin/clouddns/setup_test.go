package clouddns

import (
	"fmt"
	"io/ioutil"
	"os"
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
	// Writing dummy JSON data in the temporary JSON file
	testjsonfile, err := ioutil.TempFile(".", "testjsonfile")
	jsonData := []byte(`{"type": "service_account"}`)
	err = ioutil.WriteFile(testjsonfile.Name(), jsonData, 0644)

	defer os.Remove(testjsonfile.Name())
	if err != nil {
		log.Fatal(err)
	}
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", testjsonfile.Name())
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

	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
	    fallthrough
	}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Unexpected errors: %v", err)
	}
	os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
			credentials
	 		upstream 1.2.3.4
		}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}
	credstring := `clouddns testproject:testzone {
		credentials ` + testjsonfile.Name() +
		`
		upstream 1.2.3.4
	}`
	fmt.Printf(credstring)
	c = caddy.NewTestController("dns", credstring)
	if err := setup(c, f); err != nil {
		t.Fatalf("Unexpected errors: %v", err)
	}

	c = caddy.NewTestController("dns", `clouddns testproject:testzone {
			credentials credfilepath1 credentials extra-arg
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
