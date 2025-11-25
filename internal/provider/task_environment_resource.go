package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TaskEnvironmentResource{}
var _ resource.ResourceWithImportState = &TaskEnvironmentResource{}
var _ resource.ResourceWithModifyPlan = &TaskEnvironmentResource{}

func NewTaskEnvironmentResource() resource.Resource {
	return &TaskEnvironmentResource{}
}

// TaskEnvironmentResource defines the resource implementation.
type TaskEnvironmentResource struct {
	flyte FlyteEnvironment
}

// TaskEnvironmentResourceModel describes the resource data model.
type TaskEnvironmentResourceModel struct {
	Id      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Path    types.String `tfsdk:"path"`
	Project types.String `tfsdk:"project"`
	Domain  types.String `tfsdk:"domain"`
	Version types.String `tfsdk:"version"`
	Tasks   types.List   `tfsdk:"tasks"`
}

func (r *TaskEnvironmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_task_environment"
}

func (r *TaskEnvironmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Task environment resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Task environment identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the task environment",
			},
			"path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "This points to the task Python file.",
			},
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project name",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Domain name",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"version": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Version of the task environment",
			},
			"tasks": schema.ListAttribute{
				Computed:            true,
				MarkdownDescription: "List of tasks in the environment",
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *TaskEnvironmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	_, ok := req.ProviderData.(*providerContext)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *providerContext, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}
}

func (r *TaskEnvironmentResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var plan TaskEnvironmentResourceModel
	var state TaskEnvironmentResourceModel

	if req.Plan.Raw.IsNull() {
		return
	}
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !req.State.Raw.IsNull() {
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	details, err := r.flyte.retrieveNameAndVersion(
		ctx,
		plan.Path.ValueString(),
		plan.Project.ValueString(),
		plan.Domain.ValueString(), plan.Id.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Task environment version calculation failed",
			fmt.Sprintf("Failed to calculate version for %s: %s", plan.Path.ValueString(), err),
		)
		return
	}
	tflog.Trace(ctx, "traced modify plan", map[string]interface{}{
		"name":    details.Name,
		"version": details.Version,
	})

	plan.Name = types.StringValue(details.Name)
	plan.Version = types.StringValue(details.Version)

	ts := details.Tasks
	if ts == nil {
		ts = []string{}
	}
	lv, diags := types.ListValueFrom(ctx, types.StringType, ts)
	resp.Diagnostics.Append(diags...)
	plan.Tasks = lv

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

func (r *TaskEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TaskEnvironmentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Check if the python file exists
	if _, err := os.Stat(data.Path.ValueString()); os.IsNotExist(err) {
		resp.Diagnostics.AddError(
			"Task environment path does not exist",
			fmt.Sprintf("The path %s does not exist. Please check the path and try again.", data.Path.ValueString()),
		)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TaskEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TaskEnvironmentResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TaskEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TaskEnvironmentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.flyte.uploadNewVersion(data.Path.ValueString(), data.Project.ValueString(), data.Domain.ValueString(), data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Task environment update failed",
			fmt.Sprintf("Failed to upload new version for %s: %s", data.Path.ValueString(), err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TaskEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TaskEnvironmentResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *TaskEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
