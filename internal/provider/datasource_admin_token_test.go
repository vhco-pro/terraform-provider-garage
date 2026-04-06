package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSourceAdminToken_current(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "garage_admin_token" "current" {
  current = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.garage_admin_token.current", "id"),
					resource.TestCheckResourceAttrSet("data.garage_admin_token.current", "name"),
				),
			},
		},
	})
}

func TestAccDataSourceAdminToken_byId(t *testing.T) {
	tokenName := randomName("ds-token")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAdminTokenConfig("token", tokenName) + `
data "garage_admin_token" "test" {
  id         = garage_admin_token.token.id
  depends_on = [garage_admin_token.token]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.garage_admin_token.test", "name", tokenName),
				),
			},
		},
	})
}

func TestAccDataSourceAdminTokens_basic(t *testing.T) {
	tokenName := randomName("ds-tokens")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAdminTokenConfig("token", tokenName) + `
data "garage_admin_tokens" "all" {
  depends_on = [garage_admin_token.token]
}
`,
				Check: resource.TestCheckResourceAttrSet("data.garage_admin_tokens.all", "tokens.#"),
			},
		},
	})
}
