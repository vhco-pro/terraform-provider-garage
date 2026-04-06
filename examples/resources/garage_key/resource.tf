# Auto-generate a new access key
resource "garage_key" "app" {
  name = "application-key"
}

# Import a predefined access key
resource "garage_key" "legacy" {
  id                = "GK0123456789abcdef01234567"
  secret_access_key = var.legacy_secret_key
  name              = "legacy-key"
}
