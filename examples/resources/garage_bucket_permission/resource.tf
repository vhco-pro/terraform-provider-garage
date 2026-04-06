resource "garage_bucket_permission" "app_readwrite" {
  bucket_id     = garage_bucket.data.id
  access_key_id = garage_key.app.id
  read          = true
  write         = true
  owner         = false
}
