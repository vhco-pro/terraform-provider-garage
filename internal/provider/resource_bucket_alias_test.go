package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccBucketAlias_global(t *testing.T) {
	bucketAlias := randomName("alias")
	secondAlias := randomName("alias2")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig("bucket", bucketAlias) + fmt.Sprintf(`
resource "garage_bucket_alias" "test" {
  bucket_id  = garage_bucket.bucket.id
  alias_type = "global"
  name       = %q
}
`, secondAlias),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("garage_bucket_alias.test", "id"),
					resource.TestCheckResourceAttr("garage_bucket_alias.test", "alias_type", "global"),
					resource.TestCheckResourceAttr("garage_bucket_alias.test", "name", secondAlias),
				),
			},
			{
				ResourceName:      "garage_bucket_alias.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccBucketAlias_local(t *testing.T) {
	bucketAlias := randomName("alias")
	keyName := randomName("alias-key")
	localAlias := randomName("local")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig("bucket", bucketAlias) +
					testAccKeyConfig("key", keyName) + fmt.Sprintf(`
resource "garage_bucket_alias" "test" {
  bucket_id     = garage_bucket.bucket.id
  alias_type    = "local"
  name          = %q
  access_key_id = garage_key.key.id
}
`, localAlias),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_bucket_alias.test", "alias_type", "local"),
					resource.TestCheckResourceAttr("garage_bucket_alias.test", "name", localAlias),
				),
			},
		},
	})
}
