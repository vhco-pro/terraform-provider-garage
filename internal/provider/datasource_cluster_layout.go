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
	_ datasource.DataSource              = &ClusterLayoutDataSource{}
	_ datasource.DataSourceWithConfigure = &ClusterLayoutDataSource{}
)

type ClusterLayoutDataSource struct {
	client *garage.GarageClient
}

type ClusterLayoutDataSourceModel struct {
	Version       types.Int64      `tfsdk:"version"`
	PartitionSize types.Int64      `tfsdk:"partition_size"`
	Roles         []LayoutRoleItem `tfsdk:"roles"`
}

type LayoutRoleItem struct {
	ID               types.String `tfsdk:"id"`
	Zone             types.String `tfsdk:"zone"`
	Capacity         types.Int64  `tfsdk:"capacity"`
	Tags             types.List   `tfsdk:"tags"`
	UsableCapacity   types.Int64  `tfsdk:"usable_capacity"`
	StoredPartitions types.Int64  `tfsdk:"stored_partitions"`
}

func NewClusterLayoutDataSource() datasource.DataSource {
	return &ClusterLayoutDataSource{}
}

func (d *ClusterLayoutDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ClusterLayoutDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_layout"
}

func (d *ClusterLayoutDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Read the current Garage cluster layout.",
		Attributes: map[string]schema.Attribute{
			"version": schema.Int64Attribute{
				Description: "Current layout version.",
				Computed:    true,
			},
			"partition_size": schema.Int64Attribute{
				Description: "Size in bytes of one partition.",
				Computed:    true,
			},
			"roles": schema.ListNestedAttribute{
				Description: "Current node roles.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                schema.StringAttribute{Computed: true, Description: "Node ID."},
						"zone":              schema.StringAttribute{Computed: true, Description: "Zone name."},
						"capacity":          schema.Int64Attribute{Computed: true, Description: "Assigned capacity (bytes)."},
						"tags":              schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Tags."},
						"usable_capacity":   schema.Int64Attribute{Computed: true, Description: "Usable capacity (bytes)."},
						"stored_partitions": schema.Int64Attribute{Computed: true, Description: "Number of stored partitions."},
					},
				},
			},
		},
	}
}

func (d *ClusterLayoutDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	layoutResp, err := d.client.Inner().GetClusterLayoutWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster layout", err.Error())
		return
	}
	if layoutResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading cluster layout",
			fmt.Sprintf("unexpected status: %s", layoutResp.Status()))
		return
	}

	layout := layoutResp.JSON200
	state := ClusterLayoutDataSourceModel{
		Version:       types.Int64Value(layout.Version),
		PartitionSize: types.Int64Value(layout.PartitionSize),
	}

	for _, role := range layout.Roles {
		item := LayoutRoleItem{
			ID:   types.StringValue(role.Id),
			Zone: types.StringValue(role.Zone),
		}
		if role.Capacity != nil {
			item.Capacity = types.Int64Value(*role.Capacity)
		} else {
			item.Capacity = types.Int64Null()
		}
		if role.UsableCapacity != nil {
			item.UsableCapacity = types.Int64Value(*role.UsableCapacity)
		} else {
			item.UsableCapacity = types.Int64Null()
		}
		if role.StoredPartitions != nil {
			item.StoredPartitions = types.Int64Value(*role.StoredPartitions)
		} else {
			item.StoredPartitions = types.Int64Null()
		}
		tagList, diags := types.ListValueFrom(ctx, types.StringType, role.Tags)
		resp.Diagnostics.Append(diags...)
		item.Tags = tagList
		state.Roles = append(state.Roles, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
