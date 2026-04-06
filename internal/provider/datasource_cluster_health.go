package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vhco-pro/terraform-provider-garage/internal/garage"
)

var (
	_ datasource.DataSource              = &ClusterHealthDataSource{}
	_ datasource.DataSourceWithConfigure = &ClusterHealthDataSource{}
)

type ClusterHealthDataSource struct {
	client *garage.GarageClient
}

type ClusterHealthDataSourceModel struct {
	Status           types.String `tfsdk:"status"`
	KnownNodes       types.Int64  `tfsdk:"known_nodes"`
	ConnectedNodes   types.Int64  `tfsdk:"connected_nodes"`
	StorageNodes     types.Int64  `tfsdk:"storage_nodes"`
	StorageNodesOk   types.Int64  `tfsdk:"storage_nodes_ok"`
	Partitions       types.Int64  `tfsdk:"partitions"`
	PartitionsQuorum types.Int64  `tfsdk:"partitions_quorum"`
	PartitionsAllOk  types.Int64  `tfsdk:"partitions_all_ok"`
}

func NewClusterHealthDataSource() datasource.DataSource {
	return &ClusterHealthDataSource{}
}

func (d *ClusterHealthDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*garage.GarageClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *garage.GarageClient, got: %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *ClusterHealthDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_health"
}

func (d *ClusterHealthDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Read Garage cluster health metrics.",
		Attributes: map[string]schema.Attribute{
			"status":            schema.StringAttribute{Computed: true, Description: "Health status: healthy, degraded, or unavailable."},
			"known_nodes":       schema.Int64Attribute{Computed: true, Description: "Number of known nodes."},
			"connected_nodes":   schema.Int64Attribute{Computed: true, Description: "Number of connected nodes."},
			"storage_nodes":     schema.Int64Attribute{Computed: true, Description: "Number of storage nodes."},
			"storage_nodes_ok":  schema.Int64Attribute{Computed: true, Description: "Number of healthy storage nodes."},
			"partitions":        schema.Int64Attribute{Computed: true, Description: "Total number of partitions."},
			"partitions_quorum": schema.Int64Attribute{Computed: true, Description: "Partitions with write quorum."},
			"partitions_all_ok": schema.Int64Attribute{Computed: true, Description: "Partitions with all replicas available."},
		},
	}
}

func (d *ClusterHealthDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	healthResp, err := d.client.Inner().GetClusterHealthWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster health", err.Error())
		return
	}
	if healthResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading cluster health",
			fmt.Sprintf("unexpected status: %s", healthResp.Status()))
		return
	}

	h := healthResp.JSON200
	state := ClusterHealthDataSourceModel{
		Status:           types.StringValue(h.Status),
		KnownNodes:       types.Int64Value(int64(h.KnownNodes)),
		ConnectedNodes:   types.Int64Value(int64(h.ConnectedNodes)),
		StorageNodes:     types.Int64Value(int64(h.StorageNodes)),
		StorageNodesOk:   types.Int64Value(int64(h.StorageNodesUp)),
		Partitions:       types.Int64Value(int64(h.Partitions)),
		PartitionsQuorum: types.Int64Value(int64(h.PartitionsQuorum)),
		PartitionsAllOk:  types.Int64Value(int64(h.PartitionsAllOk)),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
