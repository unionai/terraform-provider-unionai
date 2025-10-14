package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/unionai/cloud/gen/pb-go/identity"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &AppDataSource{}

func NewAppDataSource() datasource.DataSource {
	return &AppDataSource{}
}

// AppDataSource defines the data source implementation.
type AppDataSource struct {
	conn identity.AppsServiceClient
	org  string
}

// AppDataSourceModel describes the data source data model.
type AppDataSourceModel struct {
	Id                      types.String   `tfsdk:"id"`
	ClientId                types.String   `tfsdk:"client_id"`
	ClientName              types.String   `tfsdk:"client_name"`
	ClientUri               types.String   `tfsdk:"client_uri"`
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

func (d *AppDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (d *AppDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Application data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Application identifier",
				Required:            true,
			},
			"client_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Application identifier",
			},
			"client_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Human-readable name of the application",
			},
			"client_uri": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "URI of the application",
			},
			"contacts": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of contacts for the application",
			},
			"grant_types": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of OAuth 2.0 grant types the application may use",
			},
			"jwks_uri": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "URI for the application's JSON Web Key Set",
			},
			"logo_uri": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "URI that references a logo for the application",
			},
			"policy_uri": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "URI that the application provides to the end-user to read about how the profile data will be used",
			},
			"redirect_uris": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of redirect URIs for the application",
			},
			"response_types": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of OAuth 2.0 response types the application may use",
			},
			"token_endpoint_auth_method": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Authentication method for the token endpoint",
			},
			"tos_uri": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "URI that the client provides to the end-user to read about the client's terms of service",
			},
			"secret": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Application secret",
			},
		},
	}
}

func (d *AppDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.conn = identity.NewAppsServiceClient(client.conn)
	if d.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *identity.AppsServiceClient, got: %T. Please report this issue to the provider developers.", d.conn),
		)
		return
	}
	d.org = client.org
}

func (d *AppDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AppDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	app, err := d.conn.Get(ctx, &identity.GetAppRequest{
		Organization: d.org,
		ClientId:     data.Id.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading UnionAI Application",
			fmt.Sprintf("Error reading UnionAI Application %s: %s", data.Id.ValueString(), err),
		)
		return
	}

	// Map response body to model
	data.ClientId = types.StringValue(app.App.ClientId)
	data.ClientName = types.StringValue(app.App.ClientName)
	data.ClientUri = types.StringValue(app.App.ClientUri)
	data.Contacts = make([]types.String, len(app.App.Contacts))
	for i, c := range app.App.Contacts {
		data.Contacts[i] = types.StringValue(c)
	}
	data.GrantTypes = make([]types.String, len(app.App.GrantTypes))
	for i, g := range app.App.GrantTypes {
		data.GrantTypes[i] = types.StringValue(identity.GrantTypes_name[int32(g)])
	}
	data.JwksUri = types.StringValue(app.App.JwksUri)
	data.LogoUri = types.StringValue(app.App.LogoUri)
	data.PolicyUri = types.StringValue(app.App.PolicyUri)
	data.RedirectUris = make([]types.String, len(app.App.RedirectUris))
	for i, r := range app.App.RedirectUris {
		data.RedirectUris[i] = types.StringValue(r)
	}
	data.ResponseTypes = make([]types.String, len(app.App.ResponseTypes))
	for i, r := range app.App.ResponseTypes {
		data.ResponseTypes[i] = types.StringValue(identity.ResponseTypes_name[int32(r)])
	}
	data.TokenEndpointAuthMethod = types.StringValue(identity.TokenEndpointAuthMethod_name[int32(app.App.TokenEndpointAuthMethod)])
	data.TosUri = types.StringValue(app.App.TosUri)

	data.Secret = types.StringValue(app.App.ClientSecret)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
