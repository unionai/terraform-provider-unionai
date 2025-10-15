package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/unionai/cloud/gen/pb-go/authorizer"
	"github.com/unionai/cloud/gen/pb-go/common"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PolicyDataSource{}

func NewPolicyDataSource() datasource.DataSource {
	return &PolicyDataSource{}
}

// PolicyDataSource defines the data source implementation.
type PolicyDataSource struct {
	conn authorizer.AuthorizerServiceClient
	org  string
}

// PolicyDataSourceModel describes the data source data model.
type PolicyDataSourceModel struct {
	Id          types.String                `tfsdk:"id"`
	Roles       []PolicyRoleDataSourceModel `tfsdk:"roles"`
	Description types.String                `tfsdk:"description"`
}

type PolicyRoleDataSourceModel struct {
	RoleId   types.String            `tfsdk:"role_id"`
	Resource ResourceDataSourceModel `tfsdk:"resource"`
}

type ResourceDataSourceModel struct {
	OrgId     types.String `tfsdk:"org_id"`
	DomainId  types.String `tfsdk:"domain_id"`
	ProjectId types.String `tfsdk:"project_id"`
}

func (d *PolicyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (d *PolicyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Policy data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Policy identifier",
				Required:            true,
			},
			"roles": schema.ListNestedAttribute{
				MarkdownDescription: "Policy roles",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role_id": schema.StringAttribute{
							MarkdownDescription: "Role identifier",
							Computed:            true,
						},
						"resource": schema.SingleNestedAttribute{
							MarkdownDescription: "Resource name",
							Computed:            true,
							Attributes: map[string]schema.Attribute{
								"org_id": schema.StringAttribute{
									MarkdownDescription: "Org identifier",
									Computed:            true,
								},
								"domain_id": schema.StringAttribute{
									MarkdownDescription: "Domain identifier",
									Computed:            true,
								},
								"project_id": schema.StringAttribute{
									MarkdownDescription: "Project identifier",
									Computed:            true,
								},
							},
						},
					},
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Policy description",
				Computed:            true,
			},
		},
	}
}

func (d *PolicyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.conn = authorizer.NewAuthorizerServiceClient(client.conn)
	if d.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *authorizer.AuthorizerServiceClient, got: %T. Please report this issue to the provider developers.", d.conn),
		)
		return
	}
	d.org = client.org
}

func (d *PolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PolicyDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := d.conn.GetPolicy(ctx, &authorizer.GetPolicyRequest{
		Id: &common.PolicyIdentifier{
			Name:         data.Id.ValueString(),
			Organization: d.org,
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read policy, got error: %s", err))
		return
	}

	data.Description = types.StringValue(policy.Policy.Description)
	for _, b := range policy.Policy.Bindings {
		var r ResourceDataSourceModel
		if b.Resource.GetOrganization() != nil {
			r.OrgId = types.StringValue(b.Resource.GetOrganization().Name)
		}
		if b.Resource.GetDomain() != nil {
			r.DomainId = types.StringValue(b.Resource.GetDomain().Name)
		}
		if b.Resource.GetProject() != nil {
			r.ProjectId = types.StringValue(b.Resource.GetProject().Name)
		}
		data.Roles = append(data.Roles, PolicyRoleDataSourceModel{
			RoleId:   types.StringValue(b.RoleId.Name),
			Resource: r,
		})
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
