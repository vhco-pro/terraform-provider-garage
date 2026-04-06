package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vhco-pro/terraform-provider-garage/internal/garage"
)

var (
	_ datasource.DataSource              = &BucketsDataSource{}
	_ datasource.DataSourceWithConfigure = &BucketsDataSource{}
)

type BucketsDataSource struct {
	client *garage.GarageClient
}

type BucketsDataSourceModel struct {
	Buckets []BucketListItem `tfsdk:"buckets"`
}

type BucketListItem struct {
	ID            types.String `tfsdk:"id"`
	GlobalAliases types.List   `tfsdk:"global_aliases"`
	LocalAliases  types.List   `tfsdk:"local_aliases"`
}

type BucketLocalAliasModel struct {
	AccessKeyID types.String `tfsdk:"access_key_id"`
	Alias       types.String `tfsdk:"alias"`
}

func NewBucketsDataSource() datasource.DataSource {
	return &BucketsDataSource{}
}

func (d *BucketsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*garage.GarageClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *garage.GarageClient, got: %T", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *BucketsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_buckets"
}

func (d *BucketsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all Garage buckets.",
		Attributes: map[string]schema.Attribute{
			"buckets": schema.ListNestedAttribute{
				Description: "List of buckets.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Bucket identifier.",
							Computed:    true,
						},
						"global_aliases": schema.ListAttribute{
							Description: "Global aliases for this bucket.",
							Computed:    true,
							ElementType: types.StringType,
						},
						"local_aliases": schema.ListNestedAttribute{
							Description: "Local aliases for this bucket.",
							Computed:    true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"access_key_id": schema.StringAttribute{
										Description: "Access key ID that owns this alias.",
										Computed:    true,
									},
									"alias": schema.StringAttribute{
										Description: "Local alias name.",
										Computed:    true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *BucketsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	listResp, err := d.client.Inner().ListBucketsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing buckets", err.Error())
		return
	}
	if listResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error listing buckets",
			fmt.Sprintf("unexpected status: %s", listResp.Status()))
		return
	}

	var state BucketsDataSourceModel
	for _, b := range *listResp.JSON200 {
		aliases, diags := types.ListValueFrom(ctx, types.StringType, b.GlobalAliases)
		resp.Diagnostics.Append(diags...)

		var localAliases []BucketLocalAliasModel
		for _, la := range b.LocalAliases {
			localAliases = append(localAliases, BucketLocalAliasModel{
				AccessKeyID: types.StringValue(la.AccessKeyId),
				Alias:       types.StringValue(la.Alias),
			})
		}

		localAliasesList, diags := types.ListValueFrom(ctx, types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"access_key_id": types.StringType,
				"alias":         types.StringType,
			},
		}, localAliases)
		resp.Diagnostics.Append(diags...)

		state.Buckets = append(state.Buckets, BucketListItem{
			ID:            types.StringValue(b.Id),
			GlobalAliases: aliases,
			LocalAliases:  localAliasesList,
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
