package provider_test

import (
	"fmt"
)

// --- Config Builders ---

func testAccBucketConfig(name, alias string) string {
	return fmt.Sprintf(`
resource "garage_bucket" %[1]q {
  global_alias = %[2]q
}
`, name, alias)
}

func testAccBucketConfigWithQuotas(name, alias string, maxSize, maxObjects int64) string {
	return fmt.Sprintf(`
resource "garage_bucket" %[1]q {
  global_alias = %[2]q
  max_size     = %[3]d
  max_objects  = %[4]d
}
`, name, alias, maxSize, maxObjects)
}

func testAccBucketConfigWithWebsite(name, alias, indexDoc, errorDoc string) string {
	return fmt.Sprintf(`
resource "garage_bucket" %[1]q {
  global_alias   = %[2]q
  website_access = true
  index_document = %[3]q
  error_document = %[4]q
}
`, name, alias, indexDoc, errorDoc)
}

func testAccKeyConfig(name, keyName string) string {
	return fmt.Sprintf(`
resource "garage_key" %[1]q {
  name = %[2]q
}
`, name, keyName)
}

func testAccPermissionConfig(name, bucketRef, keyRef string, read, write, owner bool) string {
	return fmt.Sprintf(`
resource "garage_bucket_permission" %[1]q {
  bucket_id     = %[2]s
  access_key_id = %[3]s
  read          = %[4]t
  write         = %[5]t
  owner         = %[6]t
}
`, name, bucketRef, keyRef, read, write, owner)
}

func testAccAdminTokenConfig(name, tokenName string) string {
	return fmt.Sprintf(`
resource "garage_admin_token" %[1]q {
  name  = %[2]q
  scope = ["*"]
}
`, name, tokenName)
}

func testAccAdminTokenConfigWithScope(name, tokenName string, scope []string) string {
	scopeStr := ""
	for i, s := range scope {
		if i > 0 {
			scopeStr += ", "
		}
		scopeStr += fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf(`
resource "garage_admin_token" %[1]q {
  name  = %[2]q
  scope = [%[3]s]
}
`, name, tokenName, scopeStr)
}
