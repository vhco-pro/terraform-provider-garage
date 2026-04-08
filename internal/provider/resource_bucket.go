package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vhco-pro/terraform-provider-garage/internal/garage"
)

var (
	_ resource.Resource                = &BucketResource{}
	_ resource.ResourceWithImportState = &BucketResource{}
	_ resource.ResourceWithConfigure   = &BucketResource{}
)

type BucketResource struct {
	client *garage.GarageClient
}

type BucketResourceModel struct {
	ID                             types.String `tfsdk:"id"`
	GlobalAlias                    types.String `tfsdk:"global_alias"`
	Created                        types.String `tfsdk:"created"`
	WebsiteAccess                  types.Bool   `tfsdk:"website_access"`
	IndexDocument                  types.String `tfsdk:"index_document"`
	ErrorDocument                  types.String `tfsdk:"error_document"`
	MaxSize                        types.Int64  `tfsdk:"max_size"`
	MaxObjects                     types.Int64  `tfsdk:"max_objects"`
	Objects                        types.Int64  `tfsdk:"objects"`
	Bytes                          types.Int64  `tfsdk:"bytes"`
	UnfinishedUploads              types.Int64  `tfsdk:"unfinished_uploads"`
	UnfinishedMultipartUploads     types.Int64  `tfsdk:"unfinished_multipart_uploads"`
	UnfinishedMultipartUploadParts types.Int64  `tfsdk:"unfinished_multipart_upload_parts"`
	UnfinishedMultipartUploadBytes types.Int64  `tfsdk:"unfinished_multipart_upload_bytes"`
}

func NewBucketResource() resource.Resource {
	return &BucketResource{}
}

func (r *BucketResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*garage.GarageClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *garage.GarageClient, got: %T", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *BucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (r *BucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Garage S3 bucket.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Bucket identifier.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"global_alias": schema.StringAttribute{
				Description: "Global alias for the bucket. Must be a valid S3 bucket name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexpBucketAlias, "must be a valid S3 bucket name (lowercase, numbers, hyphens, dots)"),
				},
			},
			"created": schema.StringAttribute{
				Description: "Bucket creation timestamp.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"website_access": schema.BoolAttribute{
				Description: "Whether website access is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"index_document": schema.StringAttribute{
				Description: "Index document for website access.",
				Optional:    true,
			},
			"error_document": schema.StringAttribute{
				Description: "Error document for website access.",
				Optional:    true,
			},
			"max_size": schema.Int64Attribute{
				Description: "Maximum total size of objects in the bucket (bytes). Null means unlimited.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"max_objects": schema.Int64Attribute{
				Description: "Maximum number of objects in the bucket. Null means unlimited.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
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
		},
	}
}

func (r *BucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alias := plan.GlobalAlias.ValueString()
	createResp, err := r.client.Inner().CreateBucketWithResponse(ctx, garage.CreateBucketJSONRequestBody{
		GlobalAlias: &alias,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating bucket", err.Error())
		return
	}
	if createResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error creating bucket",
			fmt.Sprintf("unexpected status: %s, body: %s", createResp.Status(), string(createResp.Body)))
		return
	}

	bucket := createResp.JSON200
	plan.ID = types.StringValue(bucket.Id)
	plan.Created = types.StringValue(bucket.Created.Format("2006-01-02T15:04:05Z"))

	// Save state immediately so partial failure can be recovered
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Step 2: Update bucket with quotas/website if configured
	needsUpdate := false
	updateBody := garage.UpdateBucketRequestBody{}

	if !plan.WebsiteAccess.IsNull() && !plan.WebsiteAccess.IsUnknown() && plan.WebsiteAccess.ValueBool() {
		needsUpdate = true
		websiteAccess := garage.UpdateBucketWebsiteAccess{
			Enabled: true,
		}
		if !plan.IndexDocument.IsNull() {
			idx := plan.IndexDocument.ValueString()
			websiteAccess.IndexDocument = &idx
		}
		if !plan.ErrorDocument.IsNull() {
			errDoc := plan.ErrorDocument.ValueString()
			websiteAccess.ErrorDocument = &errDoc
		}
		updateBody.WebsiteAccess = &websiteAccess
	}

	if (!plan.MaxSize.IsNull() && !plan.MaxSize.IsUnknown()) || (!plan.MaxObjects.IsNull() && !plan.MaxObjects.IsUnknown()) {
		needsUpdate = true
		quotas := garage.ApiBucketQuotas{}
		if !plan.MaxSize.IsNull() && !plan.MaxSize.IsUnknown() {
			v := plan.MaxSize.ValueInt64()
			quotas.MaxSize = &v
		}
		if !plan.MaxObjects.IsNull() && !plan.MaxObjects.IsUnknown() {
			v := plan.MaxObjects.ValueInt64()
			quotas.MaxObjects = &v
		}
		updateBody.Quotas = &quotas
	}

	if needsUpdate {
		updateResp, err := r.client.Inner().UpdateBucketWithResponse(ctx,
			&garage.UpdateBucketParams{Id: bucket.Id}, updateBody)
		if err != nil {
			resp.Diagnostics.AddError("Error updating bucket after creation", err.Error())
			return
		}
		if updateResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error updating bucket after creation",
				fmt.Sprintf("unexpected status: %s", updateResp.Status()))
			return
		}
		bucket = updateResp.JSON200
	}

	r.mapBucketToState(bucket, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	getResp, err := r.client.Inner().GetBucketInfoWithResponse(ctx, &garage.GetBucketInfoParams{
		Id: &id,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading bucket", err.Error())
		return
	}

	if getResp.HTTPResponse.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if getResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading bucket",
			fmt.Sprintf("unexpected status: %s", getResp.Status()))
		return
	}

	r.mapBucketToState(getResp.JSON200, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BucketResourceModel
	var state BucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateBody := garage.UpdateBucketRequestBody{}

	// Website config — always send to ensure toggling works
	websiteEnabled := plan.WebsiteAccess.ValueBool()
	websiteAccess := garage.UpdateBucketWebsiteAccess{
		Enabled: websiteEnabled,
	}
	if websiteEnabled {
		if !plan.IndexDocument.IsNull() && !plan.IndexDocument.IsUnknown() {
			idx := plan.IndexDocument.ValueString()
			websiteAccess.IndexDocument = &idx
		}
		if !plan.ErrorDocument.IsNull() && !plan.ErrorDocument.IsUnknown() {
			errDoc := plan.ErrorDocument.ValueString()
			websiteAccess.ErrorDocument = &errDoc
		}
	}
	updateBody.WebsiteAccess = &websiteAccess

	// Quotas
	quotas := garage.ApiBucketQuotas{}
	if !plan.MaxSize.IsNull() && !plan.MaxSize.IsUnknown() {
		v := plan.MaxSize.ValueInt64()
		quotas.MaxSize = &v
	}
	if !plan.MaxObjects.IsNull() && !plan.MaxObjects.IsUnknown() {
		v := plan.MaxObjects.ValueInt64()
		quotas.MaxObjects = &v
	}
	updateBody.Quotas = &quotas

	bucketID := state.ID.ValueString()
	updateResp, err := r.client.Inner().UpdateBucketWithResponse(ctx,
		&garage.UpdateBucketParams{Id: bucketID}, updateBody)
	if err != nil {
		resp.Diagnostics.AddError("Error updating bucket", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating bucket",
			fmt.Sprintf("unexpected status: %s", updateResp.Status()))
		return
	}

	r.mapBucketToState(updateResp.JSON200, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Inner().DeleteBucketWithResponse(ctx, &garage.DeleteBucketParams{
		Id: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting bucket", err.Error())
		return
	}

	switch statusCode := deleteResp.HTTPResponse.StatusCode; statusCode {
	case 204, 200:
		// Success
	case 404:
		// Already gone
	case 400:
		resp.Diagnostics.AddError("Cannot delete bucket",
			"Bucket is not empty. Delete all objects before destroying the bucket resource.")
	default:
		resp.Diagnostics.AddError("Error deleting bucket",
			fmt.Sprintf("unexpected status: %s, body: %s", deleteResp.Status(), string(deleteResp.Body)))
	}
}

func (r *BucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *BucketResource) mapBucketToState(bucket *garage.GetBucketInfoResponse, state *BucketResourceModel) {
	state.ID = types.StringValue(bucket.Id)
	state.Created = types.StringValue(bucket.Created.Format("2006-01-02T15:04:05Z"))
	state.WebsiteAccess = types.BoolValue(bucket.WebsiteAccess)
	state.Objects = types.Int64Value(bucket.Objects)
	state.Bytes = types.Int64Value(bucket.Bytes)
	state.UnfinishedUploads = types.Int64Value(bucket.UnfinishedUploads)
	state.UnfinishedMultipartUploads = types.Int64Value(bucket.UnfinishedMultipartUploads)
	state.UnfinishedMultipartUploadParts = types.Int64Value(bucket.UnfinishedMultipartUploadParts)
	state.UnfinishedMultipartUploadBytes = types.Int64Value(bucket.UnfinishedMultipartUploadBytes)

	if bucket.WebsiteConfig != nil {
		state.IndexDocument = types.StringValue(bucket.WebsiteConfig.IndexDocument)
		if bucket.WebsiteConfig.ErrorDocument != nil {
			state.ErrorDocument = types.StringValue(*bucket.WebsiteConfig.ErrorDocument)
		} else {
			state.ErrorDocument = types.StringNull()
		}
	} else {
		state.IndexDocument = types.StringNull()
		state.ErrorDocument = types.StringNull()
	}

	if bucket.Quotas.MaxSize != nil {
		state.MaxSize = types.Int64Value(*bucket.Quotas.MaxSize)
	} else {
		state.MaxSize = types.Int64Null()
	}
	if bucket.Quotas.MaxObjects != nil {
		state.MaxObjects = types.Int64Value(*bucket.Quotas.MaxObjects)
	} else {
		state.MaxObjects = types.Int64Null()
	}

	if len(bucket.GlobalAliases) > 0 {
		state.GlobalAlias = types.StringValue(bucket.GlobalAliases[0])
	}
}
