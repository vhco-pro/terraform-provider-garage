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
	_ datasource.DataSource              = &KeyDataSource{}
	_ datasource.DataSourceWithConfigure = &KeyDataSource{}
)

type KeyDataSource struct {
	client *garage.GarageClient
}

type KeyDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Expiration   types.String `tfsdk:"expiration"`
	Expired      types.Bool   `tfsdk:"expired"`
	Created      types.String `tfsdk:"created"`
	CreateBucket types.Bool   `tfsdk:"create_bucket"`
}

func NewKeyDataSource() datasource.DataSource {
	return &KeyDataSource{}
}

func (d *KeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *KeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_key"
}

func (d *KeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a Garage access key by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Access key ID.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Friendly name of the key.",
				Computed:    true,
			},
			"expiration": schema.StringAttribute{
				Description: "Expiration date (RFC 3339).",
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
			"create_bucket": schema.BoolAttribute{
				Description: "Whether this key is allowed to create buckets.",
				Computed:    true,
			},
		},
	}
}

func (d *KeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config KeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := config.ID.ValueString()
	getResp, err := d.client.Inner().GetKeyInfoWithResponse(ctx, &garage.GetKeyInfoParams{
		Id: &id,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading key", err.Error())
		return
	}
	if getResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading key",
			fmt.Sprintf("unexpected status: %s", getResp.Status()))
		return
	}

	key := getResp.JSON200
	config.ID = types.StringValue(key.AccessKeyId)
	config.Name = types.StringValue(key.Name)
	config.Expired = types.BoolValue(key.Expired)
	if key.Created != nil {
		config.Created = types.StringValue(key.Created.Format(time.RFC3339))
	}
	if key.Expiration != nil {
		config.Expiration = types.StringValue(key.Expiration.Format(time.RFC3339))
	} else {
		config.Expiration = types.StringNull()
	}
	if key.Permissions.CreateBucket != nil {
		config.CreateBucket = types.BoolValue(*key.Permissions.CreateBucket)
	} else {
		config.CreateBucket = types.BoolValue(false)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
