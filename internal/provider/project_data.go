package provider

import (
	"context"
	"fmt"

	"github.com/flyteorg/flyte/flyteidl/gen/pb-go/flyteidl/admin"
	"github.com/flyteorg/flyte/flyteidl/gen/pb-go/flyteidl/service"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ProjectDataSource{}

func NewProjectDataSource() datasource.DataSource {
	return &ProjectDataSource{}
}

// ProjectDataSource defines the data source implementation.
type ProjectDataSource struct {
	conn service.AdminServiceClient
}

// ProjectDataSourceModel describes the data source data model.
type ProjectDataSourceModel struct {
	Id          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	DomainIds   types.Set    `tfsdk:"domain_ids"`
	State       types.String `tfsdk:"state"`
}

func (d *ProjectDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Project data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Project identifier",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Project name",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Project description",
				Computed:            true,
			},
			"domain_ids": schema.SetAttribute{
				MarkdownDescription: "Project domain identifiers",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Project state",
				Computed:            true,
			},
		},
	}
}

func (d *ProjectDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*providerContext)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *providerContext, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.conn = service.NewAdminServiceClient(client.conn)
	if d.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *service.AdminServiceClient, got: %T. Please report this issue to the provider developers.", d.conn),
		)
		return
	}
}

func (d *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	project_id_filter := Filters{
		FieldSelector: fmt.Sprintf("eq(project.identifier,%s)", data.Id.ValueString()),
		Limit:         1,
	}

	projects, err := d.conn.ListProjects(context.Background(), &admin.ProjectListRequest{Filters: project_id_filter.FieldSelector, Limit: uint32(project_id_filter.Limit)})
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch projects", err.Error())
		return
	}
	tflog.Trace(ctx, "ListProjects response", map[string]interface{}{"projects": projects})

	if len(projects.Projects) == 0 {
		resp.Diagnostics.AddError("Project not found", fmt.Sprintf("Project with ID %s not found", data.Id.String()))
		return
	}
	project := projects.Projects[0]

	tflog.Trace(ctx, "Project", map[string]interface{}{"project": project.Id, "target": data.Id.String()})

	data.Name = types.StringValue(project.Name)
	if project.Description != "" {
		data.Description = types.StringValue(project.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.DomainIds = convertArrayToSetGetter(project.Domains, func(domain *admin.Domain) string { return domain.Id })
	data.State = types.StringValue(admin.Project_ProjectState_name[int32(project.State)])

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
