data "garage_admin_tokens" "all" {}

output "token_names" {
  value = data.garage_admin_tokens.all.tokens[*].name
}
