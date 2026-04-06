package provider

import (
	"context"
	"fmt"

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
	_ resource.Resource                = &BucketAliasResource{}
	_ resource.ResourceWithImportState = &BucketAliasResource{}
	_ resource.ResourceWithConfigure   = &BucketAliasResource{}
)

type BucketAliasResource struct {
	client *garage.GarageClient
}

type BucketAliasResourceModel struct {
	ID          types.String `tfsdk:"id"`
	BucketID    types.String `tfsdk:"bucket_id"`
	AliasType   types.String `tfsdk:"alias_type"`
	Name        types.String `tfsdk:"name"`
	AccessKeyID types.String `tfsdk:"access_key_id"`
}

func NewBucketAliasResource() resource.Resource {
	return &BucketAliasResource{}
}

func (r *BucketAliasResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BucketAliasResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket_alias"
}

func (r *BucketAliasResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Garage bucket alias (global or local).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier.",
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
			"alias_type": schema.StringAttribute{
				Description: "Type of alias: 'global' or 'local'.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("global", "local"),
				},
			},
			"name": schema.StringAttribute{
				Description: "Alias name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexpBucketAlias, "must be a valid S3 bucket name"),
				},
			},
			"access_key_id": schema.StringAttribute{
				Description: "Access key ID for local aliases. Required when alias_type is 'local'.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexpKeyID, "must be a valid Garage key ID (GK + 24 hex chars)"),
				},
			},
		},
	}
}

func (r *BucketAliasResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BucketAliasResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.AliasType.ValueString() == "local" && plan.AccessKeyID.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_key_id"),
			"Missing access_key_id",
			"access_key_id is required when alias_type is 'local'.",
		)
		return
	}

	var body garage.BucketAliasEnum
	if plan.AliasType.ValueString() == "global" {
		err := body.FromBucketAliasEnum0(garage.BucketAliasEnum0{
			BucketId:    plan.BucketID.ValueString(),
			GlobalAlias: plan.Name.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Error constructing alias request", err.Error())
			return
		}
	} else {
		err := body.FromBucketAliasEnum1(garage.BucketAliasEnum1{
			BucketId:    plan.BucketID.ValueString(),
			AccessKeyId: plan.AccessKeyID.ValueString(),
			LocalAlias:  plan.Name.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Error constructing alias request", err.Error())
			return
		}
	}

	addResp, err := r.client.Inner().AddBucketAliasWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error adding bucket alias", err.Error())
		return
	}
	if addResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error adding bucket alias",
			fmt.Sprintf("unexpected status: %s, body: %s", addResp.Status(), string(addResp.Body)))
		return
	}

	plan.ID = types.StringValue(r.compositeID(&plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketAliasResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BucketAliasResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.BucketID.ValueString()
	getResp, err := r.client.Inner().GetBucketInfoWithResponse(ctx, &garage.GetBucketInfoParams{
		Id: &id,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading bucket for alias", err.Error())
		return
	}
	if getResp.HTTPResponse.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading bucket for alias",
			fmt.Sprintf("unexpected status: %s", getResp.Status()))
		return
	}

	bucket := getResp.JSON200
	aliasName := state.Name.ValueString()

	if state.AliasType.ValueString() == "global" {
		found := false
		for _, a := range bucket.GlobalAliases {
			if a == aliasName {
				found = true
				break
			}
		}
		if !found {
			resp.State.RemoveResource(ctx)
			return
		}
	} else {
		found := false
		for _, k := range bucket.Keys {
			if k.AccessKeyId == state.AccessKeyID.ValueString() {
				for _, la := range k.BucketLocalAliases {
					if la == aliasName {
						found = true
						break
					}
				}
			}
		}
		if !found {
			resp.State.RemoveResource(ctx)
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BucketAliasResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "All attributes require replacement.")
}

func (r *BucketAliasResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BucketAliasResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var body garage.BucketAliasEnum
	if state.AliasType.ValueString() == "global" {
		err := body.FromBucketAliasEnum0(garage.BucketAliasEnum0{
			BucketId:    state.BucketID.ValueString(),
			GlobalAlias: state.Name.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Error constructing alias removal request", err.Error())
			return
		}
	} else {
		err := body.FromBucketAliasEnum1(garage.BucketAliasEnum1{
			BucketId:    state.BucketID.ValueString(),
			AccessKeyId: state.AccessKeyID.ValueString(),
			LocalAlias:  state.Name.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Error constructing alias removal request", err.Error())
			return
		}
	}

	removeResp, err := r.client.Inner().RemoveBucketAliasWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error removing bucket alias", err.Error())
		return
	}
	if removeResp.HTTPResponse.StatusCode != 200 && removeResp.HTTPResponse.StatusCode != 204 {
		if removeResp.HTTPResponse.StatusCode == 404 {
			return // Already gone
		}
		resp.Diagnostics.AddError("Error removing bucket alias",
			fmt.Sprintf("unexpected status: %s", removeResp.Status()))
	}
}

func (r *BucketAliasResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parsed, err := parseAliasID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket_id"), parsed.BucketID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("alias_type"), parsed.AliasType)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parsed.Name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	if parsed.AliasType == "local" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("access_key_id"), parsed.AccessKeyID)...)
	}
}

func (r *BucketAliasResource) compositeID(model *BucketAliasResourceModel) string {
	return formatAliasID(aliasID{
		BucketID:    model.BucketID.ValueString(),
		AliasType:   model.AliasType.ValueString(),
		Name:        model.Name.ValueString(),
		AccessKeyID: model.AccessKeyID.ValueString(),
	})
}
