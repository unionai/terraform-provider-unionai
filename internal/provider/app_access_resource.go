package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/unionai/cloud/gen/pb-go/authorizer"
	"github.com/unionai/cloud/gen/pb-go/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AppAccessResource{}
var _ resource.ResourceWithImportState = &AppAccessResource{}

func NewAppAccessResource() resource.Resource {
	return &AppAccessResource{}
}

// AppAccessResource defines the resource implementation.
type AppAccessResource struct {
	conn authorizer.AuthorizerServiceClient
	org  string
}

// AppAccessResourceModel describes the resource data model.
type AppAccessResourceModel struct {
	Policy types.String `tfsdk:"policy"`
	App    types.String `tfsdk:"app"`
}

func (r *AppAccessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_access"
}

func (r *AppAccessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Application access resource",

		Attributes: map[string]schema.Attribute{
			"policy": schema.StringAttribute{
				MarkdownDescription: "Policy identifier",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"app": schema.StringAttribute{
				MarkdownDescription: "Application identifier",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *AppAccessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AppAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AppAccessResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.conn.AssignIdentity(ctx, &authorizer.AssignIdentityRequest{
		Organization: r.org,
		Identity: &common.Identity{
			Principal: &common.Identity_ApplicationId{
				ApplicationId: &common.ApplicationIdentifier{
					Subject: data.App.ValueString(),
				},
			},
		},
		Assignment: &authorizer.AssignIdentityRequest_PolicyId{
			PolicyId: &common.PolicyIdentifier{
				Name:         data.Policy.ValueString(),
				Organization: r.org,
			},
		},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Application Access",
			fmt.Sprintf("Could not create application access, unexpected error: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AppAccessResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.conn.GetIdentityAssignments(ctx, &authorizer.GetIdentityAssignmentRequest{
		Organization: r.org,
		Identity: &common.Identity{
			Principal: &common.Identity_ApplicationId{
				ApplicationId: &common.ApplicationIdentifier{
					Subject: data.App.ValueString(),
				},
			},
		},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Application Access",
			fmt.Sprintf("Error reading application access for app %s: %s", data.App.ValueString(), err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AppAccessResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AppAccessResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.conn.UnassignIdentity(ctx, &authorizer.UnassignIdentityRequest{
		Organization: r.org,
		Identity: &common.Identity{
			Principal: &common.Identity_ApplicationId{
				ApplicationId: &common.ApplicationIdentifier{
					Subject: data.App.ValueString(),
				},
			},
		},
		Assignment: &authorizer.UnassignIdentityRequest_PolicyId{
			PolicyId: &common.PolicyIdentifier{
				Name:         data.Policy.ValueString(),
				Organization: r.org,
			},
		},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete application access, got error: %s", err))
		return
	}
}

func (r *AppAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
