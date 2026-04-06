# Global alias
resource "garage_bucket_alias" "cdn" {
  bucket_id  = garage_bucket.website.id
  alias_type = "global"
  name       = "cdn-alias"
}

# Local alias scoped to an access key
resource "garage_bucket_alias" "app_local" {
  bucket_id     = garage_bucket.data.id
  alias_type    = "local"
  name          = "app-data"
  access_key_id = garage_key.app.id
}
