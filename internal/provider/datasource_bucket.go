package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vhco-pro/terraform-provider-garage/internal/garage"
)

var (
	_ datasource.DataSource              = &BucketDataSource{}
	_ datasource.DataSourceWithConfigure = &BucketDataSource{}
)

type BucketDataSource struct {
	client *garage.GarageClient
}

type BucketDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	GlobalAlias   types.String `tfsdk:"global_alias"`
	Created       types.String `tfsdk:"created"`
	WebsiteAccess types.Bool   `tfsdk:"website_access"`
	IndexDocument types.String `tfsdk:"index_document"`
	ErrorDocument types.String `tfsdk:"error_document"`
	MaxSize       types.Int64  `tfsdk:"max_size"`
	MaxObjects    types.Int64  `tfsdk:"max_objects"`
	Objects       types.Int64  `tfsdk:"objects"`
	Bytes         types.Int64  `tfsdk:"bytes"`
	UnfinishedUploads              types.Int64 `tfsdk:"unfinished_uploads"`
	UnfinishedMultipartUploads     types.Int64 `tfsdk:"unfinished_multipart_uploads"`
	UnfinishedMultipartUploadParts types.Int64 `tfsdk:"unfinished_multipart_upload_parts"`
	UnfinishedMultipartUploadBytes types.Int64 `tfsdk:"unfinished_multipart_upload_bytes"`
	GlobalAliases types.List   `tfsdk:"global_aliases"`
}

func NewBucketDataSource() datasource.DataSource {
	return &BucketDataSource{}
}

func (d *BucketDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *BucketDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (d *BucketDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a Garage bucket by ID or global alias.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Bucket identifier. Exactly one of `id` or `global_alias` must be set.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("global_alias")),
				},
			},
			"global_alias": schema.StringAttribute{
				Description: "Global alias to look up.",
				Optional:    true,
				Computed:    true,
			},
			"created": schema.StringAttribute{
				Description: "Bucket creation timestamp.",
				Computed:    true,
			},
			"website_access": schema.BoolAttribute{
				Description: "Whether website access is enabled.",
				Computed:    true,
			},
			"index_document": schema.StringAttribute{
				Description: "Index document for website access.",
				Computed:    true,
			},
			"error_document": schema.StringAttribute{
				Description: "Error document for website access.",
				Computed:    true,
			},
			"max_size": schema.Int64Attribute{
				Description: "Maximum total size of objects in the bucket (bytes).",
				Computed:    true,
			},
			"max_objects": schema.Int64Attribute{
				Description: "Maximum number of objects in the bucket.",
				Computed:    true,
			},
			"objects": schema.Int64Attribute{
				Description: "Number of objects in the bucket.",
				Computed:    true,
			},
			"bytes": schema.Int64Attribute{
				Description: "Total size of objects in the bucket (bytes).",
				Computed:    true,
			},
			"unfinished_uploads": schema.Int64Attribute{
				Description: "Number of unfinished uploads.",
				Computed:    true,
			},
			"unfinished_multipart_uploads": schema.Int64Attribute{
				Description: "Number of unfinished multipart uploads.",
				Computed:    true,
			},
			"unfinished_multipart_upload_parts": schema.Int64Attribute{
				Description: "Number of parts in unfinished multipart uploads.",
				Computed:    true,
			},
			"unfinished_multipart_upload_bytes": schema.Int64Attribute{
				Description: "Total bytes in unfinished multipart uploads.",
				Computed:    true,
			},
			"global_aliases": schema.ListAttribute{
				Description: "List of all global aliases for this bucket.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *BucketDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config BucketDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &garage.GetBucketInfoParams{}
	if !config.ID.IsNull() {
		id := config.ID.ValueString()
		params.Id = &id
	}
	if !config.GlobalAlias.IsNull() {
		alias := config.GlobalAlias.ValueString()
		params.GlobalAlias = &alias
	}

	getResp, err := d.client.Inner().GetBucketInfoWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Error reading bucket", err.Error())
		return
	}
	if getResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading bucket",
			fmt.Sprintf("unexpected status: %s", getResp.Status()))
		return
	}

	bucket := getResp.JSON200
	config.ID = types.StringValue(bucket.Id)
	config.Created = types.StringValue(bucket.Created.Format("2006-01-02T15:04:05Z"))
	config.WebsiteAccess = types.BoolValue(bucket.WebsiteAccess)
	config.Objects = types.Int64Value(bucket.Objects)
	config.Bytes = types.Int64Value(bucket.Bytes)
	config.UnfinishedUploads = types.Int64Value(bucket.UnfinishedUploads)
	config.UnfinishedMultipartUploads = types.Int64Value(bucket.UnfinishedMultipartUploads)
	config.UnfinishedMultipartUploadParts = types.Int64Value(bucket.UnfinishedMultipartUploadParts)
	config.UnfinishedMultipartUploadBytes = types.Int64Value(bucket.UnfinishedMultipartUploadBytes)

	if bucket.WebsiteConfig != nil {
		config.IndexDocument = types.StringValue(bucket.WebsiteConfig.IndexDocument)
		if bucket.WebsiteConfig.ErrorDocument != nil {
			config.ErrorDocument = types.StringValue(*bucket.WebsiteConfig.ErrorDocument)
		} else {
			config.ErrorDocument = types.StringNull()
		}
	} else {
		config.IndexDocument = types.StringNull()
		config.ErrorDocument = types.StringNull()
	}

	if bucket.Quotas.MaxSize != nil {
		config.MaxSize = types.Int64Value(*bucket.Quotas.MaxSize)
	} else {
		config.MaxSize = types.Int64Null()
	}
	if bucket.Quotas.MaxObjects != nil {
		config.MaxObjects = types.Int64Value(*bucket.Quotas.MaxObjects)
	} else {
		config.MaxObjects = types.Int64Null()
	}

	if len(bucket.GlobalAliases) > 0 {
		config.GlobalAlias = types.StringValue(bucket.GlobalAliases[0])
	}

	aliases, diags := types.ListValueFrom(ctx, types.StringType, bucket.GlobalAliases)
	resp.Diagnostics.Append(diags...)
	config.GlobalAliases = aliases

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
