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
	_ datasource.DataSource              = &ClusterStatusDataSource{}
	_ datasource.DataSourceWithConfigure = &ClusterStatusDataSource{}
)

type ClusterStatusDataSource struct {
	client *garage.GarageClient
}

type ClusterStatusDataSourceModel struct {
	LayoutVersion types.Int64         `tfsdk:"layout_version"`
	Nodes         []ClusterStatusNode `tfsdk:"nodes"`
}

type ClusterStatusNode struct {
	ID            types.String `tfsdk:"id"`
	IsUp          types.Bool   `tfsdk:"is_up"`
	Draining      types.Bool   `tfsdk:"draining"`
	Addr          types.String `tfsdk:"addr"`
	Hostname      types.String `tfsdk:"hostname"`
	GarageVersion types.String `tfsdk:"garage_version"`
}

func NewClusterStatusDataSource() datasource.DataSource {
	return &ClusterStatusDataSource{}
}

func (d *ClusterStatusDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ClusterStatusDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_status"
}

func (d *ClusterStatusDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Read the current Garage cluster status.",
		Attributes: map[string]schema.Attribute{
			"layout_version": schema.Int64Attribute{Computed: true, Description: "Current layout version."},
			"nodes": schema.ListNestedAttribute{
				Description: "Cluster nodes.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":             schema.StringAttribute{Computed: true, Description: "Node ID."},
						"is_up":          schema.BoolAttribute{Computed: true, Description: "Whether the node is connected."},
						"draining":       schema.BoolAttribute{Computed: true, Description: "Whether the node is draining."},
						"addr":           schema.StringAttribute{Computed: true, Description: "Socket address."},
						"hostname":       schema.StringAttribute{Computed: true, Description: "Hostname."},
						"garage_version": schema.StringAttribute{Computed: true, Description: "Garage version."},
					},
				},
			},
		},
	}
}

func (d *ClusterStatusDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	statusResp, err := d.client.Inner().GetClusterStatusWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster status", err.Error())
		return
	}
	if statusResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading cluster status",
			fmt.Sprintf("unexpected status: %s", statusResp.Status()))
		return
	}

	s := statusResp.JSON200
	state := ClusterStatusDataSourceModel{
		LayoutVersion: types.Int64Value(s.LayoutVersion),
	}

	for _, n := range s.Nodes {
		node := ClusterStatusNode{
			ID:       types.StringValue(n.Id),
			IsUp:     types.BoolValue(n.IsUp),
			Draining: types.BoolValue(n.Draining),
		}
		if n.Addr != nil {
			node.Addr = types.StringValue(*n.Addr)
		} else {
			node.Addr = types.StringNull()
		}
		if n.Hostname != nil {
			node.Hostname = types.StringValue(*n.Hostname)
		} else {
			node.Hostname = types.StringNull()
		}
		if n.GarageVersion != nil {
			node.GarageVersion = types.StringValue(*n.GarageVersion)
		} else {
			node.GarageVersion = types.StringNull()
		}
		state.Nodes = append(state.Nodes, node)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
