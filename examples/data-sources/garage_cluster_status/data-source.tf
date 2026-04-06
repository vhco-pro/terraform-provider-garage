data "garage_cluster_status" "current" {}

output "garage_version" {
  value = data.garage_cluster_status.current.garage_version
}
