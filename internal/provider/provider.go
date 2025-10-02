package provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

// Ensure UnionProvider satisfies various provider interfaces.
var _ provider.Provider = &UnionaiProvider{}

// UnionaiProvider defines the provider implementation.
type UnionaiProvider struct {
	version string
}

// UnionaiProviderModel describes the provider data model.
type UnionaiProviderModel struct {
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Host         types.String `tfsdk:"host"`
}

type providerContext struct {
	conn *grpc.ClientConn
}

func (p *UnionaiProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "unionai"
	resp.Version = p.version
}

func (p *UnionaiProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				MarkdownDescription: "OAuth client identifier",
				Optional:            true, // they can be specified by UNIONAI_CLIENT_ID
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "OAuth client secret",
				Optional:            true, // they can be specified by UNIONAI_CLIENT_SECRET
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "Unionai host",
				Optional:            true, // they can be specified by UNIONAI_HOST
			},
		},
	}
}

func (p *UnionaiProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data UnionaiProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	host := os.Getenv("UNIONAI_HOST")
	if host == "" {
		host = data.Host.ValueString()
	}
	if host == "" {
		resp.Diagnostics.AddError(
			"Union.ai host is required",
			"Union.ai host can be specified by UNIONAI_HOST or host attribute.",
		)
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("host: %s", host))
	clientId := os.Getenv("UNIONAI_CLIENT_ID")
	if clientId == "" {
		clientId = data.ClientID.ValueString()
	}
	if clientId == "" {
		resp.Diagnostics.AddError(
			"Union.ai client_id is required",
			"Union.ai client_id can be specified by UNIONAI_CLIENT_ID or client_id attribute.",
		)
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("client_id: %s", clientId))
	clientSecret := os.Getenv("UNIONAI_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = data.ClientSecret.ValueString()
	}
	if clientSecret == "" {
		resp.Diagnostics.AddError(
			"Union.ai client_secret is required",
			"Union.ai client_secret can be specified by UNIONAI_CLIENT_SECRET or client_secret attribute.",
		)
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("client_secret: %s", clientSecret))

	// Get OAuth2 token source using OpenID configuration
	token, err := GetApiToken(host, clientId, clientSecret)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get OAuth2 token", err.Error())
		return
	}

	// Create gRPC connection with OAuth2 credentials
	conn, err := grpc.NewClient(
		host,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: token}),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to connect to Unionai host", err.Error())
		return
	}

	client := &providerContext{
		conn: conn,
	}
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *UnionaiProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewUserResource,
		NewRoleResource,
		NewPolicyResource,
		NewPolicyBindingResource,
		NewOAuthAppResource,
	}
}

func (p *UnionaiProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewUserDataSource,
		NewRoleDataSource,
		NewPolicyDataSource,
		NewPolicyBindingDataSource,
		NewOAuthAppDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &UnionaiProvider{
			version: version,
		}
	}
}
