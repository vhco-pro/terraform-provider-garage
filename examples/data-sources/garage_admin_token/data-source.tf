# Get current token info
data "garage_admin_token" "current" {
  current = true
}

# Look up by ID
data "garage_admin_token" "deploy" {
  id = "abc123"
}
