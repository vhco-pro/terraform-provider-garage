resource "garage_layout_node" "node1" {
  node_id  = data.garage_cluster_status.current.nodes[0].id
  zone     = "dc1"
  capacity = 1073741824 # 1 GiB
  tags     = ["storage", "fast"]
}

# -----------------------------------------------------------------------------
# Bootstrapping a multi-node cluster from scratch
#
# Garage requires every node listed in the layout to satisfy the replication
# factor before an apply succeeds. Because each `garage_layout_node` resource
# performs its own stage+apply cycle, chain them with `depends_on` so they are
# assigned one at a time, and ensure the cluster's replication_factor is
# reached before the first apply is allowed to settle.
#
# This pattern works for clusters deployed on Kubernetes via Helm, where the
# Garage pods are queried through the `garage_cluster_status` data source and
# then iterated by stable hostname.
# -----------------------------------------------------------------------------

locals {
  # Logical storage budget per node, in bytes.
  garage_layout_capacity_bytes = 50 * 1024 * 1024 * 1024

  # Treat all nodes as a single failure zone when they share one datacenter.
  garage_layout_zone = "example"
}

data "garage_cluster_status" "garage" {}

locals {
  garage_node_ids_by_hostname = {
    for node in data.garage_cluster_status.garage.nodes : node.hostname => node.id
  }
}

resource "garage_layout_node" "garage_0" {
  node_id  = local.garage_node_ids_by_hostname["garage-0"]
  zone     = local.garage_layout_zone
  capacity = local.garage_layout_capacity_bytes
}

resource "garage_layout_node" "garage_1" {
  depends_on = [garage_layout_node.garage_0]

  node_id  = local.garage_node_ids_by_hostname["garage-1"]
  zone     = local.garage_layout_zone
  capacity = local.garage_layout_capacity_bytes
}

resource "garage_layout_node" "garage_2" {
  depends_on = [garage_layout_node.garage_1]

  node_id  = local.garage_node_ids_by_hostname["garage-2"]
  zone     = local.garage_layout_zone
  capacity = local.garage_layout_capacity_bytes
}
