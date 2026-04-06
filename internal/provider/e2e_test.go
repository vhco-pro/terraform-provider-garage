package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccE2E_BucketKeyPermission(t *testing.T) {
	bucketAlias := randomName("e2e-bkp")
	keyName := randomName("e2e-key")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create bucket + key + read+write permission
			{
				Config: fmt.Sprintf(`
resource "garage_key" "app" {
  name = %[1]q
}

resource "garage_bucket" "data" {
  global_alias = %[2]q
  max_size     = 1073741824
  max_objects  = 10000
}

resource "garage_bucket_permission" "app_data" {
  bucket_id     = garage_bucket.data.id
  access_key_id = garage_key.app.id
  read          = true
  write         = true
  owner         = false
}
`, keyName, bucketAlias),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("garage_bucket.data", "id"),
					resource.TestCheckResourceAttrSet("garage_key.app", "id"),
					resource.TestCheckResourceAttr("garage_bucket_permission.app_data", "read", "true"),
					resource.TestCheckResourceAttr("garage_bucket_permission.app_data", "write", "true"),
					resource.TestCheckResourceAttr("garage_bucket_permission.app_data", "owner", "false"),
				),
			},
			// Step 2: Promote to owner
			{
				Config: fmt.Sprintf(`
resource "garage_key" "app" {
  name = %[1]q
}

resource "garage_bucket" "data" {
  global_alias = %[2]q
  max_size     = 1073741824
  max_objects  = 10000
}

resource "garage_bucket_permission" "app_data" {
  bucket_id     = garage_bucket.data.id
  access_key_id = garage_key.app.id
  read          = true
  write         = true
  owner         = true
}
`, keyName, bucketAlias),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_bucket_permission.app_data", "owner", "true"),
				),
			},
		},
	})
}

func TestAccE2E_AdminTokenLifecycle(t *testing.T) {
	tokenName := randomName("e2e-token")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create token with scope
			{
				Config: fmt.Sprintf(`
resource "garage_admin_token" "deploy" {
  name  = %[1]q
  scope = ["GetClusterStatus", "GetClusterHealth", "ListBuckets"]
}

data "garage_admin_tokens" "all" {
  depends_on = [garage_admin_token.deploy]
}
`, tokenName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("garage_admin_token.deploy", "id"),
					resource.TestCheckResourceAttrSet("garage_admin_token.deploy", "secret_token"),
					resource.TestCheckResourceAttr("garage_admin_token.deploy", "name", tokenName),
					resource.TestCheckResourceAttrSet("data.garage_admin_tokens.all", "tokens.#"),
				),
			},
			// Step 2: Update scope to wildcard
			{
				Config: fmt.Sprintf(`
resource "garage_admin_token" "deploy" {
  name  = %[1]q
  scope = ["*"]
}

data "garage_admin_tokens" "all" {
  depends_on = [garage_admin_token.deploy]
}
`, tokenName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_admin_token.deploy", "scope.0", "*"),
				),
			},
		},
	})
}
