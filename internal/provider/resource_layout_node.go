package provider

import (
	"context"
	"fmt"
	"strings"
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
	_ resource.Resource                = &LayoutNodeResource{}
	_ resource.ResourceWithImportState = &LayoutNodeResource{}
	_ resource.ResourceWithConfigure   = &LayoutNodeResource{}
)

const layoutMaxRetries = 3

type LayoutNodeResource struct {
	client *garage.GarageClient
}

type LayoutNodeResourceModel struct {
	ID            types.String `tfsdk:"id"`
	NodeID        types.String `tfsdk:"node_id"`
	Zone          types.String `tfsdk:"zone"`
	Capacity      types.Int64  `tfsdk:"capacity"`
	Tags          types.List   `tfsdk:"tags"`
	LayoutVersion types.Int64  `tfsdk:"layout_version"`
}

func NewLayoutNodeResource() resource.Resource {
	return &LayoutNodeResource{}
}

func (r *LayoutNodeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *LayoutNodeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_layout_node"
}

func (r *LayoutNodeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a node's role in the Garage cluster layout. Changes are staged and applied (two-phase commit).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Node identifier (same as node_id).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"node_id": schema.StringAttribute{
				Description: "64-character hex node identifier.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexpNodeID, "must be a 64-character hex string"),
				},
			},
			"zone": schema.StringAttribute{
				Description: "Zone name for the node.",
				Required:    true,
			},
			"capacity": schema.Int64Attribute{
				Description: "Storage capacity in bytes. Null for gateway nodes.",
				Optional:    true,
			},
			"tags": schema.ListAttribute{
				Description: "Tags for the node.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"layout_version": schema.Int64Attribute{
				Description: "Layout version after the last apply.",
				Computed:    true,
			},
		},
	}
}

func (r *LayoutNodeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LayoutNodeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var tags []string
	if !plan.Tags.IsNull() {
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	version, err := r.stageAndApply(ctx, plan.NodeID.ValueString(), plan.Zone.ValueString(), plan.Capacity, tags, false)
	if err != nil {
		resp.Diagnostics.AddError("Error creating layout node", err.Error())
		return
	}

	plan.ID = plan.NodeID
	plan.LayoutVersion = types.Int64Value(version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LayoutNodeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LayoutNodeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	layoutResp, err := r.client.Inner().GetClusterLayoutWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster layout", err.Error())
		return
	}
	if layoutResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading cluster layout",
			fmt.Sprintf("unexpected status: %s", layoutResp.Status()))
		return
	}

	nodeID := state.NodeID.ValueString()
	for _, role := range layoutResp.JSON200.Roles {
		if role.Id == nodeID {
			state.Zone = types.StringValue(role.Zone)
			if role.Capacity != nil {
				state.Capacity = types.Int64Value(*role.Capacity)
			} else {
				state.Capacity = types.Int64Null()
			}
			tagList, diags := types.ListValueFrom(ctx, types.StringType, role.Tags)
			resp.Diagnostics.Append(diags...)
			state.Tags = tagList
			state.LayoutVersion = types.Int64Value(layoutResp.JSON200.Version)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}

	// Node not found in layout
	resp.State.RemoveResource(ctx)
}

func (r *LayoutNodeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LayoutNodeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var tags []string
	if !plan.Tags.IsNull() {
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	version, err := r.stageAndApply(ctx, plan.NodeID.ValueString(), plan.Zone.ValueString(), plan.Capacity, tags, false)
	if err != nil {
		resp.Diagnostics.AddError("Error updating layout node", err.Error())
		return
	}

	plan.ID = plan.NodeID
	plan.LayoutVersion = types.Int64Value(version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LayoutNodeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state LayoutNodeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.stageAndApply(ctx, state.NodeID.ValueString(), "", types.Int64Null(), nil, true)
	if err != nil {
		// If Garage rejects the removal because it would violate the replication
		// factor (e.g., removing the last node), revert the staged change and
		// let Terraform remove the resource from state with a warning.
		if strings.Contains(err.Error(), "smaller than the replication factor") {
			// Revert any staged layout change
			_, _ = r.client.Inner().RevertClusterLayoutWithResponse(ctx)
			resp.Diagnostics.AddWarning(
				"Layout node not physically removed",
				fmt.Sprintf("Garage rejected the removal because it would violate the replication factor. "+
					"The node remains in the cluster layout but has been removed from Terraform state. "+
					"Original error: %s", err.Error()),
			)
			return
		}
		resp.Diagnostics.AddError("Error removing layout node", err.Error())
	}
}

func (r *LayoutNodeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("node_id"), req.ID)...)
}

// stageAndApply performs the two-phase layout change with version conflict retry.
func (r *LayoutNodeResource) stageAndApply(ctx context.Context, nodeID, zone string, capacity types.Int64, tags []string, remove bool) (int64, error) {
	for attempt := 0; attempt <= layoutMaxRetries; attempt++ {
		// Get current layout version
		layoutResp, err := r.client.Inner().GetClusterLayoutWithResponse(ctx)
		if err != nil {
			return 0, fmt.Errorf("getting cluster layout: %w", err)
		}
		if layoutResp.JSON200 == nil {
			return 0, fmt.Errorf("getting cluster layout: unexpected status %s", layoutResp.Status())
		}

		currentVersion := layoutResp.JSON200.Version

		// Stage the change
		var roleChange garage.NodeRoleChangeRequest
		if remove {
			err = roleChange.FromNodeRoleChangeRequest0(garage.NodeRoleChangeRequest0{
				Id:     nodeID,
				Remove: true,
			})
		} else {
			req1 := garage.NodeRoleChangeRequest1{
				Id:   nodeID,
				Zone: zone,
				Tags: tags,
			}
			if !capacity.IsNull() {
				v := capacity.ValueInt64()
				req1.Capacity = &v
			}
			err = roleChange.FromNodeRoleChangeRequest1(req1)
		}
		if err != nil {
			return 0, fmt.Errorf("constructing role change: %w", err)
		}

		roles := []garage.NodeRoleChangeRequest{roleChange}
		stageResp, err := r.client.Inner().UpdateClusterLayoutWithResponse(ctx, garage.UpdateClusterLayoutJSONRequestBody{
			Roles: &roles,
		})
		if err != nil {
			return 0, fmt.Errorf("staging layout change: %w", err)
		}
		if stageResp.HTTPResponse.StatusCode != 200 {
			return 0, fmt.Errorf("staging layout change: unexpected status %s", stageResp.Status())
		}

		// Apply with version N+1
		newVersion := currentVersion + 1
		applyResp, err := r.client.Inner().ApplyClusterLayoutWithResponse(ctx, garage.ApplyClusterLayoutJSONRequestBody{
			Version: newVersion,
		})
		if err != nil {
			return 0, fmt.Errorf("applying layout: %w", err)
		}

		if applyResp.HTTPResponse.StatusCode == 409 && attempt < layoutMaxRetries {
			// Version conflict — retry
			time.Sleep(time.Duration(1<<attempt) * time.Second)
			continue
		}

		if applyResp.JSON200 == nil {
			return 0, fmt.Errorf("applying layout: unexpected status %s, body: %s",
				applyResp.Status(), string(applyResp.Body))
		}

		return applyResp.JSON200.Layout.Version, nil
	}

	return 0, fmt.Errorf("layout apply failed after %d retries due to version conflicts", layoutMaxRetries)
}
