resource "garage_layout_node" "node1" {
  node_id  = data.garage_cluster_status.current.nodes[0].id
  zone     = "dc1"
  capacity = 1073741824  # 1 GiB
  tags     = ["storage", "fast"]
}
