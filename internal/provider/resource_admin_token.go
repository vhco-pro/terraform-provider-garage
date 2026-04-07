package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vhco-pro/terraform-provider-garage/internal/garage"
)

var (
	_ resource.Resource                = &AdminTokenResource{}
	_ resource.ResourceWithImportState = &AdminTokenResource{}
	_ resource.ResourceWithConfigure   = &AdminTokenResource{}
)

type AdminTokenResource struct {
	client *garage.GarageClient
}

type AdminTokenResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	SecretToken  types.String `tfsdk:"secret_token"`
	Expiration   types.String `tfsdk:"expiration"`
	NeverExpires types.Bool   `tfsdk:"never_expires"`
	Scope        types.List   `tfsdk:"scope"`
	Expired      types.Bool   `tfsdk:"expired"`
	Created      types.String `tfsdk:"created"`
}

func NewAdminTokenResource() resource.Resource {
	return &AdminTokenResource{}
}

func (r *AdminTokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AdminTokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_admin_token"
}

func (r *AdminTokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Garage admin API token.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Admin token identifier.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the admin token.",
				Required:    true,
			},
			"secret_token": schema.StringAttribute{
				Description: "The secret bearer token. Only available at creation time.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"expiration": schema.StringAttribute{
				Description: "Expiration date (RFC 3339). Mutually exclusive with never_expires.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("never_expires")),
				},
			},
			"never_expires": schema.BoolAttribute{
				Description: "Set to true to make the token never expire.",
				Optional:    true,
				Computed:    true,
			},
			"scope": schema.ListAttribute{
				Description: "List of allowed admin API endpoint names, or [\"*\"] for all.",
				Required:    true,
				ElementType: types.StringType,
			},
			"expired": schema.BoolAttribute{
				Description: "Whether the token has expired.",
				Computed:    true,
			},
			"created": schema.StringAttribute{
				Description: "Token creation timestamp.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *AdminTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AdminTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scope []string
	resp.Diagnostics.Append(plan.Scope.ElementsAs(ctx, &scope, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	createBody := garage.CreateAdminTokenJSONRequestBody{
		Name:  &name,
		Scope: &scope,
	}

	if !plan.Expiration.IsNull() && !plan.Expiration.IsUnknown() {
		t, err := time.Parse(time.RFC3339, plan.Expiration.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid expiration format", err.Error())
			return
		}
		createBody.Expiration = &t
	}
	if !plan.NeverExpires.IsNull() && !plan.NeverExpires.IsUnknown() && plan.NeverExpires.ValueBool() {
		ne := true
		createBody.NeverExpires = &ne
	}

	createResp, err := r.client.Inner().CreateAdminTokenWithResponse(ctx, createBody)
	if err != nil {
		resp.Diagnostics.AddError("Error creating admin token", err.Error())
		return
	}
	if createResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error creating admin token",
			fmt.Sprintf("unexpected status: %s", createResp.Status()))
		return
	}

	token := createResp.JSON200
	if token.Id != nil {
		plan.ID = types.StringValue(*token.Id)
	}
	plan.SecretToken = types.StringValue(token.SecretToken)
	plan.Name = types.StringValue(token.Name)
	plan.Expired = types.BoolValue(token.Expired)

	if token.Created != nil {
		plan.Created = types.StringValue(token.Created.Format(time.RFC3339))
	}
	if token.Expiration != nil {
		plan.Expiration = types.StringValue(token.Expiration.Format(time.RFC3339))
		plan.NeverExpires = types.BoolValue(false)
	} else {
		plan.Expiration = types.StringNull()
		plan.NeverExpires = types.BoolValue(true)
	}

	scopeList, diags := types.ListValueFrom(ctx, types.StringType, token.Scope)
	resp.Diagnostics.Append(diags...)
	plan.Scope = scopeList

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AdminTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AdminTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	getResp, err := r.client.Inner().GetAdminTokenInfoWithResponse(ctx, &garage.GetAdminTokenInfoParams{
		Id: &id,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading admin token", err.Error())
		return
	}
	if getResp.HTTPResponse.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading admin token",
			fmt.Sprintf("unexpected status: %s", getResp.Status()))
		return
	}

	token := getResp.JSON200
	if token.Id != nil {
		state.ID = types.StringValue(*token.Id)
	}
	state.Name = types.StringValue(token.Name)
	state.Expired = types.BoolValue(token.Expired)

	if token.Created != nil {
		state.Created = types.StringValue(token.Created.Format(time.RFC3339))
	}
	if token.Expiration != nil {
		state.Expiration = types.StringValue(token.Expiration.Format(time.RFC3339))
		state.NeverExpires = types.BoolValue(false)
	} else {
		state.Expiration = types.StringNull()
		state.NeverExpires = types.BoolValue(true)
	}

	scopeList, diags := types.ListValueFrom(ctx, types.StringType, token.Scope)
	resp.Diagnostics.Append(diags...)
	state.Scope = scopeList

	// secret_token is preserved from state (not returned by read)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AdminTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AdminTokenResourceModel
	var state AdminTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scope []string
	resp.Diagnostics.Append(plan.Scope.ElementsAs(ctx, &scope, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	updateBody := garage.UpdateAdminTokenJSONRequestBody{
		Name:  &name,
		Scope: &scope,
	}

	if !plan.Expiration.IsNull() && !plan.Expiration.IsUnknown() {
		t, err := time.Parse(time.RFC3339, plan.Expiration.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid expiration format", err.Error())
			return
		}
		updateBody.Expiration = &t
	}
	if !plan.NeverExpires.IsNull() && !plan.NeverExpires.IsUnknown() && plan.NeverExpires.ValueBool() {
		ne := true
		updateBody.NeverExpires = &ne
	}

	tokenID := state.ID.ValueString()
	updateResp, err := r.client.Inner().UpdateAdminTokenWithResponse(ctx,
		&garage.UpdateAdminTokenParams{Id: tokenID}, updateBody)
	if err != nil {
		resp.Diagnostics.AddError("Error updating admin token", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating admin token",
			fmt.Sprintf("unexpected status: %s", updateResp.Status()))
		return
	}

	token := updateResp.JSON200
	if token.Id != nil {
		plan.ID = types.StringValue(*token.Id)
	}
	plan.Name = types.StringValue(token.Name)
	plan.Expired = types.BoolValue(token.Expired)
	// Preserve secret from state
	plan.SecretToken = state.SecretToken

	if token.Created != nil {
		plan.Created = types.StringValue(token.Created.Format(time.RFC3339))
	}
	if token.Expiration != nil {
		plan.Expiration = types.StringValue(token.Expiration.Format(time.RFC3339))
		plan.NeverExpires = types.BoolValue(false)
	} else {
		plan.Expiration = types.StringNull()
		plan.NeverExpires = types.BoolValue(true)
	}

	scopeList, diags := types.ListValueFrom(ctx, types.StringType, token.Scope)
	resp.Diagnostics.Append(diags...)
	plan.Scope = scopeList

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AdminTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AdminTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Inner().DeleteAdminTokenWithResponse(ctx, &garage.DeleteAdminTokenParams{
		Id: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting admin token", err.Error())
		return
	}
	statusCode := deleteResp.HTTPResponse.StatusCode
	if statusCode != 200 && statusCode != 204 && statusCode != 404 {
		resp.Diagnostics.AddError("Error deleting admin token",
			fmt.Sprintf("unexpected status: %s", deleteResp.Status()))
	}
}

func (r *AdminTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.AddWarning(
		"Secret token unavailable after import",
		"The secret_token attribute will be null after import. It is only available at creation time.",
	)
}
