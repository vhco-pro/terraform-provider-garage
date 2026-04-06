package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccBucketPermission_readOnly(t *testing.T) {
	alias := randomName("perm")
	keyName := randomName("perm-key")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig("bucket", alias) +
					testAccKeyConfig("key", keyName) +
					testAccPermissionConfig("test", "garage_bucket.bucket.id", "garage_key.key.id", true, false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_bucket_permission.test", "read", "true"),
					resource.TestCheckResourceAttr("garage_bucket_permission.test", "write", "false"),
					resource.TestCheckResourceAttr("garage_bucket_permission.test", "owner", "false"),
				),
			},
			{
				ResourceName:      "garage_bucket_permission.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccBucketPermission_readWrite(t *testing.T) {
	alias := randomName("perm")
	keyName := randomName("perm-key")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig("bucket", alias) +
					testAccKeyConfig("key", keyName) +
					testAccPermissionConfig("test", "garage_bucket.bucket.id", "garage_key.key.id", true, true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_bucket_permission.test", "read", "true"),
					resource.TestCheckResourceAttr("garage_bucket_permission.test", "write", "true"),
				),
			},
		},
	})
}

func TestAccBucketPermission_addWrite(t *testing.T) {
	alias := randomName("perm")
	keyName := randomName("perm-key")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig("bucket", alias) +
					testAccKeyConfig("key", keyName) +
					testAccPermissionConfig("test", "garage_bucket.bucket.id", "garage_key.key.id", true, false, false),
				Check: resource.TestCheckResourceAttr("garage_bucket_permission.test", "write", "false"),
			},
			{
				Config: testAccBucketConfig("bucket", alias) +
					testAccKeyConfig("key", keyName) +
					testAccPermissionConfig("test", "garage_bucket.bucket.id", "garage_key.key.id", true, true, false),
				Check: resource.TestCheckResourceAttr("garage_bucket_permission.test", "write", "true"),
			},
		},
	})
}

func TestAccBucketPermission_revokeOwner(t *testing.T) {
	alias := randomName("perm")
	keyName := randomName("perm-key")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig("bucket", alias) +
					testAccKeyConfig("key", keyName) +
					testAccPermissionConfig("test", "garage_bucket.bucket.id", "garage_key.key.id", true, true, true),
				Check: resource.TestCheckResourceAttr("garage_bucket_permission.test", "owner", "true"),
			},
			{
				Config: testAccBucketConfig("bucket", alias) +
					testAccKeyConfig("key", keyName) +
					testAccPermissionConfig("test", "garage_bucket.bucket.id", "garage_key.key.id", true, true, false),
				Check: resource.TestCheckResourceAttr("garage_bucket_permission.test", "owner", "false"),
			},
		},
	})
}
