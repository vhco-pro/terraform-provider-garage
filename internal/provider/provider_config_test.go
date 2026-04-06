package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProvider_configFromBlock(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "garage" {
  endpoint = "http://localhost:3903"
  token    = "test-admin-token"
}

data "garage_cluster_health" "test" {}
`,
				Check: resource.TestCheckResourceAttrSet("data.garage_cluster_health.test", "status"),
			},
		},
	})
}

func TestAccProvider_invalidEndpoint(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "garage" {
  endpoint = "not-a-url"
  token    = "test-token"
}

data "garage_cluster_health" "test" {}
`,
				ExpectError: regexp.MustCompile(`.*`),
			},
		},
	})
}
