package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccKey_basic(t *testing.T) {
	keyName := randomName("key")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKeyConfig("test", keyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("garage_key.test", "id"),
					resource.TestCheckResourceAttr("garage_key.test", "name", keyName),
					resource.TestCheckResourceAttrSet("garage_key.test", "secret_access_key"),
				),
			},
			{
				ResourceName:            "garage_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret_access_key"},
			},
		},
	})
}

func TestAccKey_updateName(t *testing.T) {
	keyName := randomName("key")
	updatedName := randomName("key")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKeyConfig("test", keyName),
				Check:  resource.TestCheckResourceAttr("garage_key.test", "name", keyName),
			},
			{
				Config: testAccKeyConfig("test", updatedName),
				Check:  resource.TestCheckResourceAttr("garage_key.test", "name", updatedName),
			},
		},
	})
}

func TestAccKey_secretPreserved(t *testing.T) {
	keyName := randomName("key")
	var secretKey string
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKeyConfig("test", keyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("garage_key.test", "secret_access_key"),
					resource.TestCheckResourceAttrWith("garage_key.test", "secret_access_key", func(v string) error {
						secretKey = v
						return nil
					}),
				),
			},
			{
				Config: testAccKeyConfig("test", keyName),
				Check: resource.TestCheckResourceAttrWith("garage_key.test", "secret_access_key", func(v string) error {
					if v != secretKey {
						return fmt.Errorf("secret_access_key changed: was %q, now %q", secretKey, v)
					}
					return nil
				}),
			},
		},
	})
}
