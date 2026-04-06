data "garage_cluster_health" "current" {}

output "cluster_status" {
  value = data.garage_cluster_health.current.status
}
