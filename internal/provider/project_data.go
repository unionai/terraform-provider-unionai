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

type ProjectState string

const (
	Project_ACTIVE           ProjectState = "ACTIVE"
	Project_ARCHIVED         ProjectState = "ARCHIVED"
	Project_SYSTEM_GENERATED ProjectState = "SYSTEM_GENERATED"
	Project_SYSTEM_ARCHIVED  ProjectState = "SYSTEM_ARCHIVED"
	Project_UNKNOWN          ProjectState = "UNKNOWN"
)

// ProjectDataSourceModel describes the data source data model.
type ProjectDataSourceModel struct {
	Id          types.String                   `tfsdk:"id"`
	Name        types.String                   `tfsdk:"name"`
	Description types.String                   `tfsdk:"description"`
	Domains     []ProjectDomainDataSourceModel `tfsdk:"domains"`
	State       types.String                   `tfsdk:"state"`
}

type ProjectDomainDataSourceModel struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
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
			"domains": schema.ListNestedAttribute{
				MarkdownDescription: "Project domains",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Project domain identifier",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Project domain name",
							Computed:            true,
						},
					},
				},
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

	projects, err := d.conn.ListProjects(context.Background(), &admin.ProjectListRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch projects", err.Error())
		return
	}
	tflog.Trace(ctx, "ListProjects response", map[string]interface{}{"projects": projects})

	found := false
	for _, project := range projects.Projects {
		tflog.Trace(ctx, "Project", map[string]interface{}{"project": project.Id, "target": data.Id.String()})
		if project.Id == data.Id.ValueString() {
			data.Name = types.StringValue(project.Name)

			if project.Description != "" {
				data.Description = types.StringValue(project.Description)
			} else {
				data.Description = types.StringNull()
			}

			data.Domains = make([]ProjectDomainDataSourceModel, 0, len(project.Domains))
			for _, domain := range project.Domains {
				data.Domains = append(data.Domains, ProjectDomainDataSourceModel{
					Id:   types.StringValue(domain.Id),
					Name: types.StringValue(domain.Name),
				})
			}

			switch project.State {
			case admin.Project_ACTIVE:
				data.State = types.StringValue(string(Project_ACTIVE))
			case admin.Project_ARCHIVED:
				data.State = types.StringValue(string(Project_ARCHIVED))
			case admin.Project_SYSTEM_GENERATED:
				data.State = types.StringValue(string(Project_SYSTEM_GENERATED))
			default:
				data.State = types.StringValue(string(Project_UNKNOWN))
			}

			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError("Project not found", fmt.Sprintf("Project with ID %s not found", data.Id.String()))
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
