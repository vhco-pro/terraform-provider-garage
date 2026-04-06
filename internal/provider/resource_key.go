package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vhco-pro/terraform-provider-garage/internal/garage"
)

var (
	_ resource.Resource                = &KeyResource{}
	_ resource.ResourceWithImportState = &KeyResource{}
	_ resource.ResourceWithConfigure   = &KeyResource{}
)

type KeyResource struct {
	client *garage.GarageClient
}

type KeyResourceModel struct {
	ID             types.String `tfsdk:"id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
	Name           types.String `tfsdk:"name"`
	Expiration     types.String `tfsdk:"expiration"`
	NeverExpires   types.Bool   `tfsdk:"never_expires"`
	Expired        types.Bool   `tfsdk:"expired"`
	Created        types.String `tfsdk:"created"`
	CreateBucket   types.Bool   `tfsdk:"create_bucket"`
}

func NewKeyResource() resource.Resource {
	return &KeyResource{}
}

func (r *KeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *KeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_key"
}

func (r *KeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Garage access key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Access key ID. If set together with secret_access_key, imports a predefined key.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexpKeyID, "must be a valid Garage key ID (GK + 24 hex chars)"),
					stringvalidator.AlsoRequires(path.MatchRoot("secret_access_key")),
				},
			},
			"secret_access_key": schema.StringAttribute{
				Description: "Secret access key. Only available at creation or import with showSecretKey.",
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("id")),
				},
			},
			"name": schema.StringAttribute{
				Description: "Friendly name for the key.",
				Optional:    true,
				Computed:    true,
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
				Description: "Set to true to make the key never expire.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"expired": schema.BoolAttribute{
				Description: "Whether the key has expired.",
				Computed:    true,
			},
			"created": schema.StringAttribute{
				Description: "Key creation timestamp.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"create_bucket": schema.BoolAttribute{
				Description: "Whether this key is allowed to create buckets.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *KeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan KeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var key *garage.GetKeyInfoResponse

	if !plan.ID.IsNull() && !plan.SecretAccessKey.IsNull() {
		// Import predefined key
		importBody := garage.ImportKeyRequest{
			AccessKeyId:    plan.ID.ValueString(),
			SecretAccessKey: plan.SecretAccessKey.ValueString(),
		}
		if !plan.Name.IsNull() {
			name := plan.Name.ValueString()
			importBody.Name = &name
		}
		importResp, err := r.client.Inner().ImportKeyWithResponse(ctx, importBody)
		if err != nil {
			resp.Diagnostics.AddError("Error importing key", err.Error())
			return
		}
		if importResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error importing key",
				fmt.Sprintf("unexpected status: %s", importResp.Status()))
			return
		}
		key = importResp.JSON200
	} else {
		// Create new key
		createBody := garage.CreateKeyJSONRequestBody{}
		if !plan.Name.IsNull() {
			name := plan.Name.ValueString()
			createBody.Name = &name
		}
		createResp, err := r.client.Inner().CreateKeyWithResponse(ctx, createBody)
		if err != nil {
			resp.Diagnostics.AddError("Error creating key", err.Error())
			return
		}
		if createResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error creating key",
				fmt.Sprintf("unexpected status: %s", createResp.Status()))
			return
		}
		key = createResp.JSON200
	}

	plan.ID = types.StringValue(key.AccessKeyId)
	if key.SecretAccessKey != nil {
		plan.SecretAccessKey = types.StringValue(*key.SecretAccessKey)
	}

	// Apply updates if needed (expiration, neverExpires, createBucket)
	needsUpdate := false
	updateBody := garage.UpdateKeyRequestBody{}

	if !plan.Name.IsNull() {
		name := plan.Name.ValueString()
		updateBody.Name = &name
		needsUpdate = true
	}
	if !plan.Expiration.IsNull() {
		t, err := time.Parse(time.RFC3339, plan.Expiration.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid expiration format", err.Error())
			return
		}
		updateBody.Expiration = &t
		needsUpdate = true
	}
	if !plan.NeverExpires.IsNull() && plan.NeverExpires.ValueBool() {
		ne := true
		updateBody.NeverExpires = &ne
		needsUpdate = true
	}
	if !plan.CreateBucket.IsNull() {
		cb := plan.CreateBucket.ValueBool()
		if cb {
			updateBody.Allow = &garage.KeyPerm{CreateBucket: &cb}
		} else {
			updateBody.Deny = &garage.KeyPerm{CreateBucket: &cb}
		}
		needsUpdate = true
	}

	if needsUpdate {
		updateResp, err := r.client.Inner().UpdateKeyWithResponse(ctx,
			&garage.UpdateKeyParams{Id: key.AccessKeyId}, updateBody)
		if err != nil {
			resp.Diagnostics.AddError("Error updating key after creation", err.Error())
			return
		}
		if updateResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error updating key after creation",
				fmt.Sprintf("unexpected status: %s", updateResp.Status()))
			return
		}
		key = updateResp.JSON200
		// Restore secret since update response may not include it
		if key.SecretAccessKey == nil && !plan.SecretAccessKey.IsNull() {
			// Keep the value from create/import
		} else if key.SecretAccessKey != nil {
			plan.SecretAccessKey = types.StringValue(*key.SecretAccessKey)
		}
	}

	r.mapKeyToState(key, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *KeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state KeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	showSecret := true
	getResp, err := r.client.Inner().GetKeyInfoWithResponse(ctx, &garage.GetKeyInfoParams{
		Id:            &id,
		ShowSecretKey: &showSecret,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading key", err.Error())
		return
	}
	if getResp.HTTPResponse.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading key",
			fmt.Sprintf("unexpected status: %s", getResp.Status()))
		return
	}

	r.mapKeyToState(getResp.JSON200, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *KeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan KeyResourceModel
	var state KeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateBody := garage.UpdateKeyRequestBody{}

	if !plan.Name.IsNull() {
		name := plan.Name.ValueString()
		updateBody.Name = &name
	}
	if !plan.Expiration.IsNull() {
		t, err := time.Parse(time.RFC3339, plan.Expiration.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid expiration format", err.Error())
			return
		}
		updateBody.Expiration = &t
	}
	if !plan.NeverExpires.IsNull() && plan.NeverExpires.ValueBool() {
		ne := true
		updateBody.NeverExpires = &ne
	}
	if !plan.CreateBucket.IsNull() {
		cb := plan.CreateBucket.ValueBool()
		ncb := !cb
		if cb {
			updateBody.Allow = &garage.KeyPerm{CreateBucket: &cb}
		} else {
			updateBody.Deny = &garage.KeyPerm{CreateBucket: &ncb}
		}
	}

	keyID := state.ID.ValueString()
	updateResp, err := r.client.Inner().UpdateKeyWithResponse(ctx,
		&garage.UpdateKeyParams{Id: keyID}, updateBody)
	if err != nil {
		resp.Diagnostics.AddError("Error updating key", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating key",
			fmt.Sprintf("unexpected status: %s", updateResp.Status()))
		return
	}

	r.mapKeyToState(updateResp.JSON200, &plan)
	// Preserve secret from state (update may not return it)
	if plan.SecretAccessKey.IsNull() || plan.SecretAccessKey.IsUnknown() {
		plan.SecretAccessKey = state.SecretAccessKey
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *KeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state KeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Inner().DeleteKeyWithResponse(ctx, &garage.DeleteKeyParams{
		Id: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting key", err.Error())
		return
	}

	statusCode := deleteResp.HTTPResponse.StatusCode
	if statusCode != 200 && statusCode != 204 && statusCode != 404 {
		resp.Diagnostics.AddError("Error deleting key",
			fmt.Sprintf("unexpected status: %s", deleteResp.Status()))
	}
}

func (r *KeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *KeyResource) mapKeyToState(key *garage.GetKeyInfoResponse, state *KeyResourceModel) {
	state.ID = types.StringValue(key.AccessKeyId)
	state.Name = types.StringValue(key.Name)
	state.Expired = types.BoolValue(key.Expired)

	if key.SecretAccessKey != nil {
		state.SecretAccessKey = types.StringValue(*key.SecretAccessKey)
	}
	if key.Created != nil {
		state.Created = types.StringValue(key.Created.Format(time.RFC3339))
	}
	if key.Expiration != nil {
		state.Expiration = types.StringValue(key.Expiration.Format(time.RFC3339))
		state.NeverExpires = types.BoolValue(false)
	} else {
		state.Expiration = types.StringNull()
		state.NeverExpires = types.BoolValue(true)
	}

	if key.Permissions.CreateBucket != nil {
		state.CreateBucket = types.BoolValue(*key.Permissions.CreateBucket)
	} else {
		state.CreateBucket = types.BoolValue(false)
	}
}
