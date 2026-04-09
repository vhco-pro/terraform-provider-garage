package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLayoutNode_basic(t *testing.T) {
	// This test uses the existing Garage node (CI configures a single node).
	// Step 1: Read node ID from cluster status, then manage it via layout_node.
	// Step 2: Update tags.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLayoutNodeConfig("dc1", 1073741824, `["storage"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("garage_layout_node.test", "id"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "zone", "dc1"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "capacity", "1073741824"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.#", "1"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.0", "storage"),
				),
			},
			{
				Config: testAccLayoutNodeConfig("dc1", 1073741824, `["storage", "primary"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.#", "2"),
					resource.TestCheckResourceAttr("garage_layout_node.test", "tags.1", "primary"),
				),
			},
		},
	})
}

func TestAccLayoutNode_updateCapacity(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLayoutNodeConfig("dc1", 1073741824, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_layout_node.test", "capacity", "1073741824"),
				),
			},
			{
				Config: testAccLayoutNodeConfig("dc1", 2147483648, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("garage_layout_node.test", "capacity", "2147483648"),
				),
			},
		},
	})
}

func TestAccLayoutNode_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLayoutNodeConfig("dc1", 1073741824, `[]`),
			},
			{
				ResourceName:      "garage_layout_node.test",
				ImportState:       true,
				ImportStateVerify: true,
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
