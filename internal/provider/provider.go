package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure UnionProvider satisfies various provider interfaces.
var _ provider.Provider = &UnionaiProvider{}

// UnionaiProvider defines the provider implementation.
type UnionaiProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// UnionaiProviderModel describes the provider data model.
type UnionaiProviderModel struct {
	ApiKey types.String `tfsdk:"api_key"`
}

func (p *UnionaiProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "unionai"
	resp.Version = p.version
}

func (p *UnionaiProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "API key for authentication",
				Required:            true,
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

	// Configuration values are now available.
	// if data.Endpoint.IsNull() { /* ... */ }

	// Example client configuration for data sources and resources
	client := http.DefaultClient
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
	}
}

func (p *UnionaiProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewUserDataSource,
		NewRoleDataSource,
		NewPolicyDataSource,
		NewPolicyBindingDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &UnionaiProvider{
			version: version,
		}
	}
}
