package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSourceKey_byId(t *testing.T) {
	keyName := randomName("ds-key")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKeyConfig("key", keyName) + `
data "garage_key" "test" {
  id         = garage_key.key.id
  depends_on = [garage_key.key]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.garage_key.test", "name", keyName),
					resource.TestCheckResourceAttrSet("data.garage_key.test", "id"),
				),
			},
		},
	})
}

func TestAccDataSourceKeys_basic(t *testing.T) {
	keyName := randomName("ds-keys")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKeyConfig("key", keyName) + `
data "garage_keys" "all" {
  depends_on = [garage_key.key]
}
`,
				Check: resource.TestCheckResourceAttrSet("data.garage_keys.all", "keys.#"),
			},
		},
	})
}
