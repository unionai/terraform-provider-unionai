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
var _ resource.Resource = &UserAccessResource{}
var _ resource.ResourceWithImportState = &UserAccessResource{}

func NewUserAccessResource() resource.Resource {
	return &UserAccessResource{}
}

// UserAccessResource defines the resource implementation.
type UserAccessResource struct {
	conn authorizer.AuthorizerServiceClient
	org  string
}

// UserAccessResourceModel describes the resource data model.
type UserAccessResourceModel struct {
	Policy types.String `tfsdk:"policy"`
	User   types.String `tfsdk:"user"`
}

func (r *UserAccessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_access"
}

func (r *UserAccessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "User access resource",

		Attributes: map[string]schema.Attribute{
			"policy": schema.StringAttribute{
				MarkdownDescription: "Policy identifier",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "User identifier",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *UserAccessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserAccessResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.conn.AssignIdentity(ctx, &authorizer.AssignIdentityRequest{
		Organization: r.org,
		Identity: &common.Identity{
			Principal: &common.Identity_UserId{
				UserId: &common.UserIdentifier{
					Subject: data.User.ValueString(),
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
			"Error Creating User Access",
			fmt.Sprintf("Could not create user access, unexpected error: %s", err.Error()),
		)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserAccessResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.conn.GetIdentityAssignments(ctx, &authorizer.GetIdentityAssignmentRequest{
		Organization: r.org,
		Identity: &common.Identity{
			Principal: &common.Identity_UserId{
				UserId: &common.UserIdentifier{
					Subject: data.User.ValueString(),
				},
			},
		},
	})
	if err != nil {
		// Catch gRPC error if the assignment is not found
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading User Access",
			fmt.Sprintf("Error reading user access for user %s: %s", data.User.ValueString(), err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserAccessResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserAccessResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.conn.UnassignIdentity(ctx, &authorizer.UnassignIdentityRequest{
		Organization: r.org,
		Identity: &common.Identity{
			Principal: &common.Identity_UserId{
				UserId: &common.UserIdentifier{
					Subject: data.User.ValueString(),
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete user access, got error: %s", err))
		return
	}
}

func (r *UserAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
