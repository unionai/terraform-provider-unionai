package provider

import (
	"context"
	"crypto/tls"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	ApiKey types.String `tfsdk:"api_key"`
}

type providerContext struct {
	conn *grpc.ClientConn
	org  string
}

func (p *UnionaiProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "unionai"
	resp.Version = p.version
}

func (p *UnionaiProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Unionai API key",
				Optional:            true, // they can be specified by UNIONAI_API_KEY
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

	apiKey := os.Getenv("UNIONAI_API_KEY")
	if apiKey == "" {
		apiKey = data.ApiKey.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Union.ai api_key is required",
			"Union.ai api_key can be specified by UNIONAI_API_KEY or api_key attribute.",
		)
		return
	}

	// Get OAuth2 token source using OpenID configuration
	token, host, err := GetApiToken(apiKey)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get OAuth2 token", err.Error())
		return
	}

	// Create gRPC connection with OAuth2 credentials
	conn, err := grpc.NewClient(
		*host,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: token}),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to connect to Unionai host", err.Error())
		return
	}

	client := &providerContext{
		conn: conn,
		org:  strings.Split(strings.TrimPrefix(strings.TrimPrefix(*host, "https://"), "dns:///"), ".")[0],
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
