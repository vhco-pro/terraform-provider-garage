package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccBucket_basic(t *testing.T) {
	alias := randomName("bucket")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig("test", alias),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("garage_bucket.test", "id"),
					resource.TestCheckResourceAttr("garage_bucket.test", "global_alias", alias),
					resource.TestCheckResourceAttrSet("garage_bucket.test", "created"),
				),
			},
			{
				ResourceName:      "garage_bucket.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccBucket_website(t *testing.T) {
	alias := randomName("bucket")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfigWithWebsite("test", alias, "index.html", "error.html"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_bucket.test", "website_access", "true"),
					resource.TestCheckResourceAttr("garage_bucket.test", "index_document", "index.html"),
					resource.TestCheckResourceAttr("garage_bucket.test", "error_document", "error.html"),
				),
			},
		},
	})
}

func TestAccBucket_quotas(t *testing.T) {
	alias := randomName("bucket")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfigWithQuotas("test", alias, 1073741824, 10000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_bucket.test", "max_size", "1073741824"),
					resource.TestCheckResourceAttr("garage_bucket.test", "max_objects", "10000"),
				),
			},
		},
	})
}

func TestAccBucket_updateQuotas(t *testing.T) {
	alias := randomName("bucket")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfigWithQuotas("test", alias, 1073741824, 10000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_bucket.test", "max_size", "1073741824"),
				),
			},
			{
				Config: testAccBucketConfigWithQuotas("test", alias, 2147483648, 20000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_bucket.test", "max_size", "2147483648"),
					resource.TestCheckResourceAttr("garage_bucket.test", "max_objects", "20000"),
				),
			},
		},
	})
}

func TestAccBucket_updateWebsiteToggle(t *testing.T) {
	alias := randomName("bucket")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfigWithWebsite("test", alias, "index.html", "error.html"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_bucket.test", "website_access", "true"),
				),
			},
			{
				Config: testAccBucketConfig("test", alias),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_bucket.test", "website_access", "false"),
				),
			},
		},
	})
}

func TestAccBucket_invalidAliasUppercase(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig("test", "My-Bucket-Invalid"),
				ExpectError: regexp.MustCompile(`.*`),
			},
		},
	})
}
