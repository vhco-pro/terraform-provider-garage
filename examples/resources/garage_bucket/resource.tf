resource "garage_bucket" "website" {
  global_alias   = "my-website"
  website_access = true
  index_document = "index.html"
  error_document = "error.html"
  max_size       = 5368709120  # 5 GiB
  max_objects    = 100000
}
