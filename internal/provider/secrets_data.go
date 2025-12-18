package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/flyteorg/flyte/v2/gen/go/flyteidl2/secret"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &SecretsDataSource{}

func NewSecretsDataSource() datasource.DataSource {
	return &SecretsDataSource{}
}

// SecretsDataSource defines the data source implementation.
type SecretsDataSource struct {
	conn secret.SecretServiceClient
	org  string
}

// SecretsDataSourceModel describes the data source data model.
type SecretsDataSourceModel struct {
	Organization types.String `tfsdk:"organization"`
	Domain       types.String `tfsdk:"domain"`
	Project      types.String `tfsdk:"project"`
	Secrets      types.List   `tfsdk:"secrets"`
}

// SecretIdentifierModel describes a secret identifier.
type SecretIdentifierModel struct {
	Name         types.String `tfsdk:"name"`
	Organization types.String `tfsdk:"organization"`
	Domain       types.String `tfsdk:"domain"`
	Project      types.String `tfsdk:"project"`
}

func (d *SecretsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secrets"
}

func (d *SecretsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Secrets data source - returns list of secrets with their identifiers",

		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Organization scope for listing secrets. Defaults to provider organization",
			},
			"domain": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Domain scope for listing secrets",
			},
			"project": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project scope for listing secrets",
			},
			"secrets": schema.ListNestedAttribute{
				MarkdownDescription: "List of secrets with their identifiers",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Secret name",
						},
						"organization": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Organization scope",
						},
						"domain": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Domain scope",
						},
						"project": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Project scope",
						},
					},
				},
			},
		},
	}
}

func (d *SecretsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.conn = secret.NewSecretServiceClient(client.conn)
	if d.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *secret.SecretServiceClient, got: %T. Please report this issue to the provider developers.", d.conn),
		)
		return
	}
	d.org = client.org
}

func (d *SecretsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SecretsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Use provider organization if not specified
	organization := data.Organization.ValueString()
	if organization == "" {
		organization = d.org
	}

	// List all secrets - handle pagination
	var allSecrets []*secret.Secret
	token := ""
	perClusterTokens := make(map[string]string)

	for {
		listReq := &secret.ListSecretsRequest{
			Organization:     organization,
			Domain:           data.Domain.ValueString(),
			Project:          data.Project.ValueString(),
			Token:            token,
			PerClusterTokens: perClusterTokens,
		}

		secretsResp, err := d.conn.ListSecrets(ctx, listReq)
		if err != nil {
			resp.Diagnostics.AddError("Failed to list secrets", err.Error())
			return
		}

		allSecrets = append(allSecrets, secretsResp.Secrets...)

		// Check if there are more pages
		if secretsResp.Token == "" && len(secretsResp.PerClusterTokens) == 0 {
			break
		}

		token = secretsResp.Token
		perClusterTokens = secretsResp.PerClusterTokens
	}

	// Build a list of secret identifiers
	secretObjects := make([]SecretIdentifierModel, 0, len(allSecrets))
	for _, s := range allSecrets {
		if s.Id != nil {
			secretObj := SecretIdentifierModel{
				Name:         types.StringValue(s.Id.Name),
				Organization: types.StringValue(s.Id.Organization),
				Domain:       types.StringValue(s.Id.Domain),
				Project:      types.StringValue(s.Id.Project),
			}
			secretObjects = append(secretObjects, secretObj)
		}
	}

	// Sort secrets by organization, domain, project, name for consistent ordering
	sort.Slice(secretObjects, func(i, j int) bool {
		if secretObjects[i].Organization.ValueString() != secretObjects[j].Organization.ValueString() {
			return secretObjects[i].Organization.ValueString() < secretObjects[j].Organization.ValueString()
		}
		if secretObjects[i].Domain.ValueString() != secretObjects[j].Domain.ValueString() {
			return secretObjects[i].Domain.ValueString() < secretObjects[j].Domain.ValueString()
		}
		if secretObjects[i].Project.ValueString() != secretObjects[j].Project.ValueString() {
			return secretObjects[i].Project.ValueString() < secretObjects[j].Project.ValueString()
		}
		return secretObjects[i].Name.ValueString() < secretObjects[j].Name.ValueString()
	})

	secretsList, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":         types.StringType,
			"organization": types.StringType,
			"domain":       types.StringType,
			"project":      types.StringType,
		},
	}, secretObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Secrets = secretsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
