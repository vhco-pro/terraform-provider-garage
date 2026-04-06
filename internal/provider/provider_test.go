package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"

	"github.com/vhco-pro/terraform-provider-garage/internal/provider"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"garage": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// testAccPreCheck validates the environment before running acceptance tests.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("GARAGE_ENDPOINT") == "" {
		t.Skip("GARAGE_ENDPOINT not set, skipping acceptance test")
	}
	if os.Getenv("GARAGE_TOKEN") == "" {
		t.Skip("GARAGE_TOKEN not set, skipping acceptance test")
	}
}

// randomName generates a unique resource name for test isolation.
func randomName(prefix string) string {
	return fmt.Sprintf("tf-test-%s-%s", prefix, acctest.RandString(8))
}
