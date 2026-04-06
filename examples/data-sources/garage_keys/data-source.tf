data "garage_keys" "all" {}

output "key_names" {
  value = data.garage_keys.all.keys[*].name
}
