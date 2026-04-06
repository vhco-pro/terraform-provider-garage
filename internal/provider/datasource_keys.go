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
	_ datasource.DataSource              = &KeysDataSource{}
	_ datasource.DataSourceWithConfigure = &KeysDataSource{}
)

type KeysDataSource struct {
	client *garage.GarageClient
}

type KeysDataSourceModel struct {
	Keys []KeyListItem `tfsdk:"keys"`
}

type KeyListItem struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Expired    types.Bool   `tfsdk:"expired"`
	Created    types.String `tfsdk:"created"`
	Expiration types.String `tfsdk:"expiration"`
}

func NewKeysDataSource() datasource.DataSource {
	return &KeysDataSource{}
}

func (d *KeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *KeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_keys"
}

func (d *KeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all Garage access keys.",
		Attributes: map[string]schema.Attribute{
			"keys": schema.ListNestedAttribute{
				Description: "List of access keys.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Access key ID.",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Friendly name of the key.",
							Computed:    true,
						},
						"expired": schema.BoolAttribute{
							Description: "Whether the key has expired.",
							Computed:    true,
						},
						"created": schema.StringAttribute{
							Description: "Key creation timestamp.",
							Computed:    true,
						},
						"expiration": schema.StringAttribute{
							Description: "Expiration date.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *KeysDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	listResp, err := d.client.Inner().ListKeysWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing keys", err.Error())
		return
	}
	if listResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error listing keys",
			fmt.Sprintf("unexpected status: %s", listResp.Status()))
		return
	}

	var state KeysDataSourceModel
	for _, k := range *listResp.JSON200 {
		item := KeyListItem{
			ID:      types.StringValue(k.Id),
			Name:    types.StringValue(k.Name),
			Expired: types.BoolValue(k.Expired),
		}
		if k.Created != nil {
			item.Created = types.StringValue(k.Created.Format(time.RFC3339))
		} else {
			item.Created = types.StringNull()
		}
		if k.Expiration != nil {
			item.Expiration = types.StringValue(k.Expiration.Format(time.RFC3339))
		} else {
			item.Expiration = types.StringNull()
		}
		state.Keys = append(state.Keys, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
