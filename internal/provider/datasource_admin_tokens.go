package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vhco-pro/terraform-provider-garage/internal/garage"
)

var (
	_ datasource.DataSource              = &AdminTokensDataSource{}
	_ datasource.DataSourceWithConfigure = &AdminTokensDataSource{}
)

type AdminTokensDataSource struct {
	client *garage.GarageClient
}

type AdminTokensDataSourceModel struct {
	Tokens []AdminTokenListItem `tfsdk:"tokens"`
}

type AdminTokenListItem struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Expiration types.String `tfsdk:"expiration"`
	Expired    types.Bool   `tfsdk:"expired"`
	Created    types.String `tfsdk:"created"`
	Scope      types.List   `tfsdk:"scope"`
}

func NewAdminTokensDataSource() datasource.DataSource {
	return &AdminTokensDataSource{}
}

func (d *AdminTokensDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AdminTokensDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_admin_tokens"
}

func (d *AdminTokensDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all Garage admin tokens.",
		Attributes: map[string]schema.Attribute{
			"tokens": schema.ListNestedAttribute{
				Description: "List of admin tokens.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":         schema.StringAttribute{Computed: true, Description: "Token ID."},
						"name":       schema.StringAttribute{Computed: true, Description: "Token name."},
						"expiration": schema.StringAttribute{Computed: true, Description: "Expiration date."},
						"expired":    schema.BoolAttribute{Computed: true, Description: "Whether expired."},
						"created":    schema.StringAttribute{Computed: true, Description: "Creation timestamp."},
						"scope": schema.ListAttribute{
							Description: "Token scope.",
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

func (d *AdminTokensDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	listResp, err := d.client.Inner().ListAdminTokensWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing admin tokens", err.Error())
		return
	}
	if listResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error listing admin tokens",
			fmt.Sprintf("unexpected status: %s", listResp.Status()))
		return
	}

	var state AdminTokensDataSourceModel
	for _, t := range *listResp.JSON200 {
		item := AdminTokenListItem{
			Name:    types.StringValue(t.Name),
			Expired: types.BoolValue(t.Expired),
		}
		if t.Id != nil {
			item.ID = types.StringValue(*t.Id)
		}
		if t.Created != nil {
			item.Created = types.StringValue(t.Created.Format(time.RFC3339))
		} else {
			item.Created = types.StringNull()
		}
		if t.Expiration != nil {
			item.Expiration = types.StringValue(t.Expiration.Format(time.RFC3339))
		} else {
			item.Expiration = types.StringNull()
		}

		scopeList, diags := types.ListValueFrom(ctx, types.StringType, t.Scope)
		resp.Diagnostics.Append(diags...)
		item.Scope = scopeList

		state.Tokens = append(state.Tokens, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
