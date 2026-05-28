package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/flyteorg/flyte/flyteidl/gen/pb-go/flyteidl/admin"
	"github.com/flyteorg/flyte/flyteidl/gen/pb-go/flyteidl/service"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ProjectDomainAttributesResource{}
var _ resource.ResourceWithImportState = &ProjectDomainAttributesResource{}

func NewProjectDomainAttributesResource() resource.Resource {
	return &ProjectDomainAttributesResource{}
}

// ProjectDomainAttributesResource manages the cluster resource attributes
// (matchable attributes of type CLUSTER_RESOURCE) for a project-domain pair.
// These attributes are substituted into the cluster resource templates that
// Flyte renders per project-domain namespace — most commonly to set a
// per-project IAM role via the defaultIamRole template variable.
type ProjectDomainAttributesResource struct {
	conn service.AdminServiceClient
	org  string
}

// ProjectDomainAttributesResourceModel describes the resource data model.
type ProjectDomainAttributesResourceModel struct {
	Id         types.String `tfsdk:"id"`
	Project    types.String `tfsdk:"project"`
	Domain     types.String `tfsdk:"domain"`
	Attributes types.Map    `tfsdk:"attributes"`
}

func (r *ProjectDomainAttributesResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_domain_attributes"
}

func (r *ProjectDomainAttributesResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Cluster resource attributes (matchable attributes of type `CLUSTER_RESOURCE`) for a project-domain pair. " +
			"The attribute map is substituted into the cluster resource templates Flyte renders for the project-domain namespace " +
			"(e.g. `defaultIamRole` to bind a per-project IAM role to the namespace's default ServiceAccount).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier, in the form `{project}/{domain}`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project": schema.StringAttribute{
				MarkdownDescription: "Project identifier the attributes apply to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain the attributes apply to (e.g. `development`, `staging`, `production`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attributes": schema.MapAttribute{
				MarkdownDescription: "Cluster resource template variables to substitute, as case-sensitive key/value pairs " +
					"(e.g. `{ defaultIamRole = \"arn:aws:iam::123456789012:role/my-role\" }`).",
				ElementType: types.StringType,
				Required:    true,
			},
		},
	}
}

func (r *ProjectDomainAttributesResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*providerContext)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *providerContext, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.conn = service.NewAdminServiceClient(client.conn)
	if r.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *service.AdminServiceClient, got: %T. Please report this issue to the provider developers.", r.conn),
		)
		return
	}
	r.org = client.org
}

// upsert applies the attribute map to the project-domain via the matchable
// attributes API. UpdateProjectDomainAttributes has upsert semantics, so it
// backs both Create and Update.
func (r *ProjectDomainAttributesResource) upsert(ctx context.Context, data *ProjectDomainAttributesResourceModel) error {
	attributes := make(map[string]string, len(data.Attributes.Elements()))
	diags := data.Attributes.ElementsAs(ctx, &attributes, false)
	if diags.HasError() {
		return fmt.Errorf("failed to read attributes: %v", diags.Errors())
	}

	_, err := r.conn.UpdateProjectDomainAttributes(ctx, &admin.ProjectDomainAttributesUpdateRequest{
		Attributes: &admin.ProjectDomainAttributes{
			Project: data.Project.ValueString(),
			Domain:  data.Domain.ValueString(),
			Org:     r.org,
			MatchingAttributes: &admin.MatchingAttributes{
				Target: &admin.MatchingAttributes_ClusterResourceAttributes{
					ClusterResourceAttributes: &admin.ClusterResourceAttributes{
						Attributes: attributes,
					},
				},
			},
		},
	})
	return err
}

func (r *ProjectDomainAttributesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectDomainAttributesResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.upsert(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to set project-domain attributes, got error: %s", err))
		return
	}

	data.Id = types.StringValue(fmt.Sprintf("%s/%s", data.Project.ValueString(), data.Domain.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectDomainAttributesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectDomainAttributesResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.conn.GetProjectDomainAttributes(ctx, &admin.ProjectDomainAttributesGetRequest{
		Project:      data.Project.ValueString(),
		Domain:       data.Domain.ValueString(),
		Org:          r.org,
		ResourceType: admin.MatchableResource_CLUSTER_RESOURCE,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read project-domain attributes, got error: %s", err))
		return
	}

	cra := got.GetAttributes().GetMatchingAttributes().GetClusterResourceAttributes()
	if cra == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	attrs, diags := types.MapValueFrom(ctx, types.StringType, cra.GetAttributes())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Attributes = attrs
	data.Id = types.StringValue(fmt.Sprintf("%s/%s", data.Project.ValueString(), data.Domain.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectDomainAttributesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectDomainAttributesResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.upsert(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update project-domain attributes, got error: %s", err))
		return
	}

	data.Id = types.StringValue(fmt.Sprintf("%s/%s", data.Project.ValueString(), data.Domain.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectDomainAttributesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectDomainAttributesResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.conn.DeleteProjectDomainAttributes(ctx, &admin.ProjectDomainAttributesDeleteRequest{
		Project:      data.Project.ValueString(),
		Domain:       data.Domain.ValueString(),
		Org:          r.org,
		ResourceType: admin.MatchableResource_CLUSTER_RESOURCE,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete project-domain attributes, got error: %s", err))
		return
	}
}

// ImportState accepts an identifier in the form "{project}/{domain}".
func (r *ProjectDomainAttributesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier in the form \"project/domain\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
