# Get info about the responding node
data "garage_node_info" "self" {}

# Get info about a specific node
data "garage_node_info" "specific" {
  node_id = "0123456789abcdef..."
}
