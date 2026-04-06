# Look up by global alias
data "garage_bucket" "website" {
  global_alias = "my-website"
}

# Look up by ID
data "garage_bucket" "specific" {
  id = "0123456789abcdef"
}
