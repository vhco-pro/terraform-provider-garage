data "garage_cluster_layout" "current" {}

output "layout_version" {
  value = data.garage_cluster_layout.current.version
}
