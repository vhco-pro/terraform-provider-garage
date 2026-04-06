package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAdminToken_basic(t *testing.T) {
	tokenName := randomName("token")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAdminTokenConfig("test", tokenName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("garage_admin_token.test", "id"),
					resource.TestCheckResourceAttr("garage_admin_token.test", "name", tokenName),
					resource.TestCheckResourceAttrSet("garage_admin_token.test", "secret_token"),
				),
			},
			{
				ResourceName:            "garage_admin_token.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret_token"},
			},
		},
	})
}

func TestAccAdminToken_updateName(t *testing.T) {
	tokenName := randomName("token")
	updatedName := randomName("token")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAdminTokenConfig("test", tokenName),
				Check:  resource.TestCheckResourceAttr("garage_admin_token.test", "name", tokenName),
			},
			{
				Config: testAccAdminTokenConfig("test", updatedName),
				Check:  resource.TestCheckResourceAttr("garage_admin_token.test", "name", updatedName),
			},
		},
	})
}

func TestAccAdminToken_updateScope(t *testing.T) {
	tokenName := randomName("token")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAdminTokenConfigWithScope("test", tokenName, []string{"GetClusterStatus"}),
				Check:  resource.TestCheckResourceAttr("garage_admin_token.test", "scope.0", "GetClusterStatus"),
			},
			{
				Config: testAccAdminTokenConfigWithScope("test", tokenName, []string{"*"}),
				Check:  resource.TestCheckResourceAttr("garage_admin_token.test", "scope.0", "*"),
			},
		},
	})
}

func TestAccAdminToken_secretPreserved(t *testing.T) {
	tokenName := randomName("token")
	var secretToken string
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAdminTokenConfig("test", tokenName),
				Check: resource.TestCheckResourceAttrWith("garage_admin_token.test", "secret_token", func(v string) error {
					secretToken = v
					return nil
				}),
			},
			{
				Config: testAccAdminTokenConfig("test", tokenName),
				Check: resource.TestCheckResourceAttrWith("garage_admin_token.test", "secret_token", func(v string) error {
					if v != secretToken {
						return fmt.Errorf("secret_token changed: was %q, now %q", secretToken, v)
					}
					return nil
				}),
			},
		},
	})
}
