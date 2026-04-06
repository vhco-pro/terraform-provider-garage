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
	_ datasource.DataSource              = &NodeInfoDataSource{}
	_ datasource.DataSourceWithConfigure = &NodeInfoDataSource{}
)

type NodeInfoDataSource struct {
	client *garage.GarageClient
}

type NodeInfoDataSourceModel struct {
	NodeID         types.String `tfsdk:"node_id"`
	GarageVersion  types.String `tfsdk:"garage_version"`
	GarageFeatures types.List   `tfsdk:"garage_features"`
	RustVersion    types.String `tfsdk:"rust_version"`
	DbEngine       types.String `tfsdk:"db_engine"`
}

func NewNodeInfoDataSource() datasource.DataSource {
	return &NodeInfoDataSource{}
}

func (d *NodeInfoDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NodeInfoDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node_info"
}

func (d *NodeInfoDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Read Garage node software information.",
		Attributes: map[string]schema.Attribute{
			"node_id": schema.StringAttribute{
				Description: "Node ID to query. Use 'self' for the responding node, or '*' for all.",
				Optional:    true,
			},
			"garage_version":  schema.StringAttribute{Computed: true, Description: "Garage version."},
			"garage_features": schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Garage features."},
			"rust_version":    schema.StringAttribute{Computed: true, Description: "Rust version."},
			"db_engine":       schema.StringAttribute{Computed: true, Description: "Database engine."},
		},
	}
}

func (d *NodeInfoDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config NodeInfoDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	node := "self"
	if !config.NodeID.IsNull() {
		node = config.NodeID.ValueString()
	}

	infoResp, err := d.client.Inner().GetNodeInfoWithResponse(ctx, &garage.GetNodeInfoParams{
		Node: node,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading node info", err.Error())
		return
	}
	if infoResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading node info",
			fmt.Sprintf("unexpected status: %s", infoResp.Status()))
		return
	}

	// GetNodeInfo returns a MultiResponse - extract the first successful entry
	multiResp := infoResp.JSON200
	if len(multiResp.Success) == 0 {
		errMsgs := ""
		for nid, msg := range multiResp.Error {
			errMsgs += fmt.Sprintf("node %s: %s; ", nid, msg)
		}
		resp.Diagnostics.AddError("No successful node info response", errMsgs)
		return
	}

	for nodeID, info := range multiResp.Success {
		config.NodeID = types.StringValue(nodeID)
		config.GarageVersion = types.StringValue(info.GarageVersion)
		config.RustVersion = types.StringValue(info.RustVersion)
		config.DbEngine = types.StringValue(info.DbEngine)

		var featuresList []string
		if info.GarageFeatures != nil {
			featuresList = *info.GarageFeatures
		}
		features, diags := types.ListValueFrom(ctx, types.StringType, featuresList)
		resp.Diagnostics.Append(diags...)
		config.GarageFeatures = features
		break // use first entry
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
