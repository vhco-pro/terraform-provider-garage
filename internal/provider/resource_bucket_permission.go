package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vhco-pro/terraform-provider-garage/internal/garage"
)

var (
	_ resource.Resource                = &BucketPermissionResource{}
	_ resource.ResourceWithImportState = &BucketPermissionResource{}
	_ resource.ResourceWithConfigure   = &BucketPermissionResource{}
)

type BucketPermissionResource struct {
	client *garage.GarageClient
}

type BucketPermissionResourceModel struct {
	ID          types.String `tfsdk:"id"`
	BucketID    types.String `tfsdk:"bucket_id"`
	AccessKeyID types.String `tfsdk:"access_key_id"`
	Read        types.Bool   `tfsdk:"read"`
	Write       types.Bool   `tfsdk:"write"`
	Owner       types.Bool   `tfsdk:"owner"`
}

func NewBucketPermissionResource() resource.Resource {
	return &BucketPermissionResource{}
}

func (r *BucketPermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BucketPermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket_permission"
}

func (r *BucketPermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages permissions for an access key on a Garage bucket.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier (bucket_id/access_key_id).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bucket_id": schema.StringAttribute{
				Description: "ID of the bucket.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"access_key_id": schema.StringAttribute{
				Description: "Access key ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"read": schema.BoolAttribute{
				Description: "Allow read access.",
				Optional:    true,
				Computed:    true,
			},
			"write": schema.BoolAttribute{
				Description: "Allow write access.",
				Optional:    true,
				Computed:    true,
			},
			"owner": schema.BoolAttribute{
				Description: "Allow owner access.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *BucketPermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BucketPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = types.StringValue(formatPermissionID(plan.BucketID.ValueString(), plan.AccessKeyID.ValueString()))

	// Grant permissions
	allowPerms := garage.ApiBucketKeyPerm{}
	hasAllow := false
	if plan.Read.ValueBool() {
		t := true
		allowPerms.Read = &t
		hasAllow = true
	}
	if plan.Write.ValueBool() {
		t := true
		allowPerms.Write = &t
		hasAllow = true
	}
	if plan.Owner.ValueBool() {
		t := true
		allowPerms.Owner = &t
		hasAllow = true
	}

	if hasAllow {
		allowResp, err := r.client.Inner().AllowBucketKeyWithResponse(ctx, garage.AllowBucketKeyJSONRequestBody{
			BucketId:    plan.BucketID.ValueString(),
			AccessKeyId: plan.AccessKeyID.ValueString(),
			Permissions: allowPerms,
		})
		if err != nil {
			resp.Diagnostics.AddError("Error granting bucket permissions", err.Error())
			return
		}
		if allowResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error granting bucket permissions",
				fmt.Sprintf("unexpected status: %s", allowResp.Status()))
			return
		}
	}

	// Read back actual permissions
	resp.Diagnostics.Append(r.readPermissions(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BucketPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.readPermissions(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BucketPermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BucketPermissionResourceModel
	var state BucketPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = state.ID

	// Compute diff: what to allow and what to deny
	current := permissionFlags{
		Read:  state.Read.ValueBool(),
		Write: state.Write.ValueBool(),
		Owner: state.Owner.ValueBool(),
	}
	desired := permissionFlags{
		Read:  plan.Read.ValueBool(),
		Write: plan.Write.ValueBool(),
		Owner: plan.Owner.ValueBool(),
	}
	allow, deny := computePermissionDiff(current, desired)

	allowPerms := garage.ApiBucketKeyPerm{}
	hasAllow := false
	if allow.Read {
		t := true
		allowPerms.Read = &t
		hasAllow = true
	}
	if allow.Write {
		t := true
		allowPerms.Write = &t
		hasAllow = true
	}
	if allow.Owner {
		t := true
		allowPerms.Owner = &t
		hasAllow = true
	}

	denyPerms := garage.ApiBucketKeyPerm{}
	hasDeny := false
	if deny.Read {
		t := true
		denyPerms.Read = &t
		hasDeny = true
	}
	if deny.Write {
		t := true
		denyPerms.Write = &t
		hasDeny = true
	}
	if deny.Owner {
		t := true
		denyPerms.Owner = &t
		hasDeny = true
	}

	if hasAllow {
		allowResp, err := r.client.Inner().AllowBucketKeyWithResponse(ctx, garage.AllowBucketKeyJSONRequestBody{
			BucketId:    plan.BucketID.ValueString(),
			AccessKeyId: plan.AccessKeyID.ValueString(),
			Permissions: allowPerms,
		})
		if err != nil {
			resp.Diagnostics.AddError("Error granting bucket permissions", err.Error())
			return
		}
		if allowResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error granting bucket permissions",
				fmt.Sprintf("unexpected status: %s", allowResp.Status()))
			return
		}
	}

	if hasDeny {
		denyResp, err := r.client.Inner().DenyBucketKeyWithResponse(ctx, garage.DenyBucketKeyJSONRequestBody{
			BucketId:    plan.BucketID.ValueString(),
			AccessKeyId: plan.AccessKeyID.ValueString(),
			Permissions: denyPerms,
		})
		if err != nil {
			resp.Diagnostics.AddError("Error revoking bucket permissions", err.Error())
			return
		}
		if denyResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error revoking bucket permissions",
				fmt.Sprintf("unexpected status: %s", denyResp.Status()))
			return
		}
	}

	// Read back actual permissions
	resp.Diagnostics.Append(r.readPermissions(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketPermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BucketPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Deny all currently-true flags
	denyPerms := garage.ApiBucketKeyPerm{}
	hasDeny := false

	if state.Read.ValueBool() {
		t := true
		denyPerms.Read = &t
		hasDeny = true
	}
	if state.Write.ValueBool() {
		t := true
		denyPerms.Write = &t
		hasDeny = true
	}
	if state.Owner.ValueBool() {
		t := true
		denyPerms.Owner = &t
		hasDeny = true
	}

	if hasDeny {
		denyResp, err := r.client.Inner().DenyBucketKeyWithResponse(ctx, garage.DenyBucketKeyJSONRequestBody{
			BucketId:    state.BucketID.ValueString(),
			AccessKeyId: state.AccessKeyID.ValueString(),
			Permissions: denyPerms,
		})
		if err != nil {
			resp.Diagnostics.AddError("Error revoking bucket permissions", err.Error())
			return
		}
		statusCode := denyResp.HTTPResponse.StatusCode
		if statusCode != 200 && statusCode != 204 && statusCode != 404 {
			resp.Diagnostics.AddError("Error revoking bucket permissions",
				fmt.Sprintf("unexpected status: %s", denyResp.Status()))
		}
	}
}

func (r *BucketPermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	bucketID, accessKeyID, err := parsePermissionID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket_id"), bucketID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("access_key_id"), accessKeyID)...)
}

func (r *BucketPermissionResource) readPermissions(ctx context.Context, model *BucketPermissionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	bucketID := model.BucketID.ValueString()
	getResp, err := r.client.Inner().GetBucketInfoWithResponse(ctx, &garage.GetBucketInfoParams{
		Id: &bucketID,
	})
	if err != nil {
		diags.AddError("Error reading bucket permissions", err.Error())
		return diags
	}
	if getResp.HTTPResponse.StatusCode == 404 {
		diags.AddError("Bucket not found", fmt.Sprintf("Bucket %s not found", bucketID))
		return diags
	}
	if getResp.JSON200 == nil {
		diags.AddError("Error reading bucket permissions",
			fmt.Sprintf("unexpected status: %s", getResp.Status()))
		return diags
	}

	keyID := model.AccessKeyID.ValueString()
	for _, k := range getResp.JSON200.Keys {
		if k.AccessKeyId == keyID {
			if k.Permissions.Read != nil {
				model.Read = types.BoolValue(*k.Permissions.Read)
			} else {
				model.Read = types.BoolValue(false)
			}
			if k.Permissions.Write != nil {
				model.Write = types.BoolValue(*k.Permissions.Write)
			} else {
				model.Write = types.BoolValue(false)
			}
			if k.Permissions.Owner != nil {
				model.Owner = types.BoolValue(*k.Permissions.Owner)
			} else {
				model.Owner = types.BoolValue(false)
			}
			return diags
		}
	}

	// Key not found in bucket permissions — no permissions
	model.Read = types.BoolValue(false)
	model.Write = types.BoolValue(false)
	model.Owner = types.BoolValue(false)
	return diags
}
