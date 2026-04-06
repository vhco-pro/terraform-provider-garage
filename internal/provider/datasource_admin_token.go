package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vhco-pro/terraform-provider-garage/internal/garage"
)

var (
	_ datasource.DataSource              = &AdminTokenDataSource{}
	_ datasource.DataSourceWithConfigure = &AdminTokenDataSource{}
)

type AdminTokenDataSource struct {
	client *garage.GarageClient
}

type AdminTokenDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	Current    types.Bool   `tfsdk:"current"`
	Name       types.String `tfsdk:"name"`
	Expiration types.String `tfsdk:"expiration"`
	Expired    types.Bool   `tfsdk:"expired"`
	Created    types.String `tfsdk:"created"`
	Scope      types.List   `tfsdk:"scope"`
}

func NewAdminTokenDataSource() datasource.DataSource {
	return &AdminTokenDataSource{}
}

func (d *AdminTokenDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AdminTokenDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_admin_token"
}

func (d *AdminTokenDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a Garage admin token by ID or get the current token.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Admin token ID. Exactly one of `id` or `current` must be set.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("current")),
				},
			},
			"current": schema.BoolAttribute{
				Description: "Set to true to get info about the current token being used.",
				Optional:    true,
			},
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
	}
}

func (d *AdminTokenDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config AdminTokenDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var token *garage.GetAdminTokenInfoResponse

	if !config.Current.IsNull() && config.Current.ValueBool() {
		getResp, err := d.client.Inner().GetCurrentAdminTokenInfoWithResponse(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error reading current admin token", err.Error())
			return
		}
		if getResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error reading current admin token",
				fmt.Sprintf("unexpected status: %s", getResp.Status()))
			return
		}
		token = getResp.JSON200
	} else {
		id := config.ID.ValueString()
		getResp, err := d.client.Inner().GetAdminTokenInfoWithResponse(ctx, &garage.GetAdminTokenInfoParams{
			Id: &id,
		})
		if err != nil {
			resp.Diagnostics.AddError("Error reading admin token", err.Error())
			return
		}
		if getResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error reading admin token",
				fmt.Sprintf("unexpected status: %s", getResp.Status()))
			return
		}
		token = getResp.JSON200
	}

	if token.Id != nil {
		config.ID = types.StringValue(*token.Id)
	}
	config.Name = types.StringValue(token.Name)
	config.Expired = types.BoolValue(token.Expired)

	if token.Created != nil {
		config.Created = types.StringValue(token.Created.Format(time.RFC3339))
	} else {
		config.Created = types.StringNull()
	}
	if token.Expiration != nil {
		config.Expiration = types.StringValue(token.Expiration.Format(time.RFC3339))
	} else {
		config.Expiration = types.StringNull()
	}

	scopeList, diags := types.ListValueFrom(ctx, types.StringType, token.Scope)
	resp.Diagnostics.Append(diags...)
	config.Scope = scopeList

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
