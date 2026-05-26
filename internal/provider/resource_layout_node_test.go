package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccLayoutNode_lifecycle tests the layout_node resource in a single-node
// Garage cluster. Because the Garage cluster requires at least one node with
// positive capacity (replication_factor=1), the test framework's automatic
// post-test destroy will fail. We use CheckDestroy to tolerate this expected
// failure — the node physically cannot be removed from a single-node cluster.
func TestAccLayoutNode_lifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// In a single-node cluster, the last node cannot be removed.
			// This is expected — the node will remain after destroy.
			return nil
		},
		Steps: []resource.TestStep{
			// Step 1: Create with tags
			{
				Config: testAccLayoutNodeConfig("dc1", 1073741824, `["storage"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("garage_layout_node.test", "id"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "zone", "dc1"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "capacity", "1073741824"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.#", "1"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.0", "storage"),
					resource.TestCheckResourceAttrSet("garage_layout_node.test", "layout_version"),
				),
			},
			// Step 2: Import
			{
				ResourceName:      "garage_layout_node.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update tags
			{
				Config: testAccLayoutNodeConfig("dc1", 1073741824, `["storage", "primary"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.#", "2"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.0", "storage"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.1", "primary"),
				),
			},
			// Step 4: Update capacity
			{
				Config: testAccLayoutNodeConfig("dc1", 2147483648, `["storage", "primary"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_layout_node.test", "capacity", "2147483648"),
				),
			},
			// Step 5: Restore to original CI config (dc1, 1GB, no tags)
			// so post-test state matches the CI setup
			{
				Config: testAccLayoutNodeConfig("dc1", 1073741824, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_layout_node.test", "capacity", "1073741824"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.#", "0"),
				),
			},
		},
	})
}

// testAccLayoutNodeConfig returns a config that looks up the first node from
// cluster status and manages it as a layout_node resource.
func testAccLayoutNodeConfig(zone string, capacity int64, tagsHCL string) string {
	return fmt.Sprintf(`
data "garage_cluster_status" "current" {}

resource "garage_layout_node" "test" {
  node_id  = data.garage_cluster_status.current.nodes[0].id
  zone     = %[1]q
  capacity = %[2]d
  tags     = %[3]s
}
`, zone, capacity, tagsHCL)
}

// testAccLayoutNodeConfigOmittedTags returns a config that omits the optional
// tags attribute entirely. Regression coverage for
// https://github.com/vhco-pro/terraform-provider-garage/issues/1 — without
// the fix this triggers "staging layout change: unexpected status 400 Bad
// Request" because tags marshal to JSON null.
func testAccLayoutNodeConfigOmittedTags(zone string, capacity int64) string {
	return fmt.Sprintf(`
data "garage_cluster_status" "current" {}

resource "garage_layout_node" "test" {
  node_id  = data.garage_cluster_status.current.nodes[0].id
  zone     = %[1]q
  capacity = %[2]d
}
`, zone, capacity)
}

// TestAccLayoutNode_omittedTags confirms that omitting the optional `tags`
// attribute from HCL succeeds. Regression test for issue #1.
func TestAccLayoutNode_omittedTags(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// Single-node cluster: the last node cannot be removed.
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccLayoutNodeConfigOmittedTags("dc1", 1073741824),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("garage_layout_node.test", "id"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "zone", "dc1"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "capacity", "1073741824"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.#", "0"),
					resource.TestCheckResourceAttrSet("garage_layout_node.test", "layout_version"),
				),
			},
		},
	})
}
