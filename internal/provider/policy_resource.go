package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/unionai/cloud/gen/pb-go/authorizer"
	"github.com/unionai/cloud/gen/pb-go/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var validDomains = []string{
	"production",
	"staging",
	"development",
}

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &PolicyResource{}
var _ resource.ResourceWithImportState = &PolicyResource{}

func NewPolicyResource() resource.Resource {
	return &PolicyResource{}
}

// PolicyResource defines the resource implementation.
type PolicyResource struct {
	conn authorizer.AuthorizerServiceClient
	org  string
}

// PolicyResourceModel describes the resource data model.
type PolicyResourceModel struct {
	Id           types.String                `tfsdk:"id"`
	Name         types.String                `tfsdk:"name"`
	Description  types.String                `tfsdk:"description"`
	Organization []PolicyRoleResourceOrg     `tfsdk:"organization"`
	Project      []PolicyRoleResourceProject `tfsdk:"project"`
	Domain       []PolicyRoleResourceDomain  `tfsdk:"domain"`
}

type PolicyRoleResourceOrg struct {
	Id     types.String `tfsdk:"id"`
	RoleId types.String `tfsdk:"role_id"`
}

type PolicyRoleResourceProject struct {
	Id      types.String `tfsdk:"id"`
	Domains types.Set    `tfsdk:"domains"`
	RoleId  types.String `tfsdk:"role_id"`
}

type PolicyRoleResourceDomain struct {
	Id     types.String `tfsdk:"id"`
	RoleId types.String `tfsdk:"role_id"`
}

func (r *PolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (r *PolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Policy resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Policy identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Policy name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Policy description",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			"organization": schema.SetNestedBlock{
				MarkdownDescription: "Organization configuration",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Organization ID",
							Required:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"role_id": schema.StringAttribute{
							MarkdownDescription: "Role ID",
							Required:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
					},
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
			"project": schema.SetNestedBlock{
				MarkdownDescription: "Project configuration",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Project ID",
							Required:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"domains": schema.SetAttribute{
							MarkdownDescription: "Domain IDs",
							Required:            true,
							ElementType:         types.StringType,
							PlanModifiers: []planmodifier.Set{
								setplanmodifier.RequiresReplace(),
							},
						},
						"role_id": schema.StringAttribute{
							MarkdownDescription: "Role ID",
							Required:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
					},
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.SetNestedBlock{
				MarkdownDescription: "Domain configuration",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Domain ID",
							Required:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"role_id": schema.StringAttribute{
							MarkdownDescription: "Role ID",
							Required:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
					},
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *PolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.conn = authorizer.NewAuthorizerServiceClient(client.conn)
	if r.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *authorizer.AuthorizerServiceClient, got: %T. Please report this issue to the provider developers.", r.conn),
		)
		return
	}
	r.org = client.org
}

func (r *PolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PolicyResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	bindings := make([]*common.PolicyBinding, 0)

	for _, org := range data.Organization {
		bindings = append(bindings, &common.PolicyBinding{
			RoleId: &common.RoleIdentifier{
				Name:         org.RoleId.ValueString(),
				Organization: r.org,
			},
			Resource: &common.Resource{
				Resource: &common.Resource_Organization{
					Organization: &common.Organization{
						Name: org.Id.ValueString(),
					},
				},
			},
		})
	}

	for _, domain := range data.Domain {
		bindings = append(bindings, &common.PolicyBinding{
			RoleId: &common.RoleIdentifier{
				Name:         domain.RoleId.ValueString(),
				Organization: r.org,
			},
			Resource: &common.Resource{
				Resource: &common.Resource_Domain{
					Domain: &common.Domain{
						Name: domain.Id.ValueString(),
						Organization: &common.Organization{
							Name: r.org,
						},
					},
				},
			},
		})
	}

	for _, project := range data.Project {
		for _, domain := range project.Domains.Elements() {
			if !slices.Contains(validDomains, domain.(types.String).ValueString()) {
				resp.Diagnostics.AddError("Invalid Domain",
					fmt.Sprintf("Domain %s is not valid. Must be one of %v", domain.(types.String).ValueString(), validDomains))
				return
			}
			bindings = append(bindings, &common.PolicyBinding{
				RoleId: &common.RoleIdentifier{
					Name:         project.RoleId.ValueString(),
					Organization: r.org,
				},
				Resource: &common.Resource{
					Resource: &common.Resource_Project{
						Project: &common.Project{
							Name: project.Id.ValueString(),
							Domain: &common.Domain{
								Name: domain.(types.String).ValueString(),
								Organization: &common.Organization{
									Name: r.org,
								},
							},
						},
					},
				},
			})
		}
	}

	if len(bindings) == 0 {
		resp.Diagnostics.AddError("Invalid Resource Scope",
			"Policy must have at least one resource scope (organization, project, or domain).")
		return
	}

	if _, err := r.conn.CreatePolicy(ctx, &authorizer.CreatePolicyRequest{
		Policy: &common.Policy{
			Id: &common.PolicyIdentifier{
				Name:         data.Name.ValueString(),
				Organization: r.org,
			},
			Description: data.Description.ValueString(),
			Bindings:    bindings,
		},
	}); err != nil {
		if status.Code(err) == codes.Internal {
			// Creation failed, but left behind. Delete it
			_, _ = r.conn.DeletePolicy(ctx, &authorizer.DeletePolicyRequest{
				Id: &common.PolicyIdentifier{
					Name:         data.Name.ValueString(),
					Organization: r.org,
				},
			})
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to create policy, got error: %s: %v", err, bindings))
			return
		}
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to create policy, got error: %s: %v", err, bindings))
		return
	}

	data.Id = types.StringValue(data.Name.ValueString())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PolicyResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use id if known, otherwise fallback to name
	if data.Id.IsUnknown() || data.Id.ValueString() == "" {
		data.Id = types.StringValue(data.Name.ValueString())
	}

	// Get the policy from the backend
	policy, err := r.conn.GetPolicy(ctx, &authorizer.GetPolicyRequest{
		Id: &common.PolicyIdentifier{
			Name:         data.Id.ValueString(),
			Organization: r.org,
		},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read policy, got error: %s", err))
		return
	}

	// Map top-level attributes
	data.Id = types.StringValue(policy.Policy.Id.Name)
	data.Name = types.StringValue(policy.Policy.Id.Name)
	if policy.Policy.Description == "" {
		data.Description = types.StringNull()
	} else {
		data.Description = types.StringValue(policy.Policy.Description)
	}

	// Reconstruct nested blocks from bindings
	orgs := make([]PolicyRoleResourceOrg, 0)
	domains := make([]PolicyRoleResourceDomain, 0)
	projDomains := make(map[string][]string) // key: roleId|projectId -> []domain

	for _, b := range policy.Policy.Bindings {
		roleId := b.RoleId.GetName()
		switch res := b.Resource.Resource.(type) {
		case *common.Resource_Organization:
			orgs = append(orgs, PolicyRoleResourceOrg{
				Id:     types.StringValue(res.Organization.GetName()),
				RoleId: types.StringValue(roleId),
			})
		case *common.Resource_Domain:
			domains = append(domains, PolicyRoleResourceDomain{
				Id:     types.StringValue(res.Domain.GetName()),
				RoleId: types.StringValue(roleId),
			})
		case *common.Resource_Project:
			projectId := res.Project.GetName()
			domainName := ""
			if res.Project.GetDomain() != nil {
				domainName = res.Project.GetDomain().GetName()
			}
			key := roleId + "|" + projectId
			if domainName != "" {
				projDomains[key] = append(projDomains[key], domainName)
			} else if _, ok := projDomains[key]; !ok {
				// track project even if no domain present
				projDomains[key] = []string{}
			}
		}
	}

	projects := make([]PolicyRoleResourceProject, 0, len(projDomains))
	for key, doms := range projDomains {
		idx := strings.Index(key, "|")
		if idx <= 0 || idx >= len(key)-1 {
			continue
		}
		roleId := key[:idx]
		projectId := key[idx+1:]

		// de-duplicate domains
		uniq := make(map[string]struct{}, len(doms))
		elements := make([]attr.Value, 0, len(doms))
		for _, d := range doms {
			if _, exists := uniq[d]; exists {
				continue
			}
			uniq[d] = struct{}{}
			elements = append(elements, types.StringValue(d))
		}
		domainSet := types.SetValueMust(types.StringType, elements)

		projects = append(projects, PolicyRoleResourceProject{
			Id:      types.StringValue(projectId),
			Domains: domainSet,
			RoleId:  types.StringValue(roleId),
		})
	}

	data.Organization = orgs
	data.Domain = domains
	data.Project = projects

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PolicyResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PolicyResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.conn.DeletePolicy(ctx, &authorizer.DeletePolicyRequest{
		Id: &common.PolicyIdentifier{
			Name:         data.Id.ValueString(),
			Organization: r.org,
		},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete policy, got error: %s", err))
		return
	}
}

func (r *PolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
