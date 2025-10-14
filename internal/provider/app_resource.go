package provider

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AppResource{}
var _ resource.ResourceWithImportState = &AppResource{}

func NewAppResource() resource.Resource {
	return &AppResource{}
}

// AppResource defines the resource implementation.
type AppResource struct {
}

// AppResourceModel describes the resource data model.
type AppResourceModel struct {
	Id                      types.String   `tfsdk:"id"`
	ClientId                types.String   `tfsdk:"client_id"`
	ClientName              types.String   `tfsdk:"client_name"`
	ClientUri               types.String   `tfsdk:"client_uri"`
	ConsentMethod           types.String   `tfsdk:"consent_method"`
	Contacts                []types.String `tfsdk:"contacts"`
	GrantTypes              []types.String `tfsdk:"grant_types"`
	JwksUri                 types.String   `tfsdk:"jwks_uri"`
	LogoUri                 types.String   `tfsdk:"logo_uri"`
	PolicyUri               types.String   `tfsdk:"policy_uri"`
	RedirectUris            []types.String `tfsdk:"redirect_uris"`
	ResponseTypes           []types.String `tfsdk:"response_types"`
	TokenEndpointAuthMethod types.String   `tfsdk:"token_endpoint_auth_method"`
	TosUri                  types.String   `tfsdk:"tos_uri"`
	Secret                  types.String   `tfsdk:"secret"`
}

func (r *AppResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (r *AppResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Application resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Application identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Application identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name of the application",
			},
			"client_uri": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "URI of the application",
			},
			"consent_method": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Consent method used by the application",
			},
			"contacts": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "List of contacts for the application",
			},
			"grant_types": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "List of OAuth 2.0 grant types the application may use",
			},
			"jwks_uri": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "URI for the application's JSON Web Key Set",
			},
			"logo_uri": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "URI that references a logo for the application",
			},
			"policy_uri": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "URI that the application provides to the end-user to read about how the profile data will be used",
			},
			"redirect_uris": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "List of redirect URIs for the application",
			},
			"response_types": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "List of OAuth 2.0 response types the application may use",
			},
			"token_endpoint_auth_method": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Authentication method for the token endpoint",
			},
			"tos_uri": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "URI that the client provides to the end-user to read about the client's terms of service",
			},
			"secret": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Application secret",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *AppResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AppResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = data.ClientId // Our ID will match the client ID which is unique

	// Fake secret. Replace with real one
	data.Secret = types.StringValue(fmt.Sprintf("fake-%d", rand.Int63()))

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AppResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read oauth app, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AppResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update oauth app, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AppResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete oauth app, got error: %s", err))
	//     return
	// }
}

func (r *AppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
