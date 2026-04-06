resource "garage_admin_token" "deploy" {
  name  = "deploy-token"
  scope = ["GetClusterStatus", "GetClusterHealth", "ListBuckets"]
}

# Token with full access and expiration
resource "garage_admin_token" "admin" {
  name       = "full-admin"
  scope      = ["*"]
  expiration = "2027-01-01T00:00:00Z"
}
