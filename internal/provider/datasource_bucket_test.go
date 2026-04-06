package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSourceBucket_byAlias(t *testing.T) {
	alias := randomName("ds-bucket")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig("bucket", alias) + fmt.Sprintf(`
data "garage_bucket" "test" {
  global_alias = %q
  depends_on   = [garage_bucket.bucket]
}
`, alias),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.garage_bucket.test", "id"),
					resource.TestCheckResourceAttrSet("data.garage_bucket.test", "created"),
				),
			},
		},
	})
}

func TestAccDataSourceBuckets_basic(t *testing.T) {
	alias := randomName("ds-buckets")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig("bucket", alias) + `
data "garage_buckets" "all" {
  depends_on = [garage_bucket.bucket]
}
`,
				Check: resource.TestCheckResourceAttrSet("data.garage_buckets.all", "buckets.#"),
			},
		},
	})
}
