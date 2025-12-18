package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/flyteorg/flyte/v2/gen/go/flyteidl2/secret"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &SecretDataSource{}

func NewSecretDataSource() datasource.DataSource {
	return &SecretDataSource{}
}

// SecretDataSource defines the data source implementation.
type SecretDataSource struct {
	conn secret.SecretServiceClient
	org  string
}

// SecretDataSourceModel describes the data source data model.
type SecretDataSourceModel struct {
	Id           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Organization types.String `tfsdk:"organization"`
	Domain       types.String `tfsdk:"domain"`
	Project      types.String `tfsdk:"project"`
	Type         types.String `tfsdk:"type"`
	CreatedTime  types.String `tfsdk:"created_time"`
}

func (d *SecretDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (d *SecretDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Secret data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Secret identifier",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Secret name",
			},
			"organization": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization scope for the secret",
			},
			"domain": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Domain scope for the secret",
			},
			"project": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project scope for the secret",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Secret type",
			},
			"created_time": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the secret was created",
			},
		},
	}
}

func (d *SecretDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SecretDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SecretDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Use provider organization if not specified
	organization := data.Organization.ValueString()
	if organization == "" {
		organization = d.org
	}

	secretResp, err := d.conn.GetSecret(ctx, &secret.GetSecretRequest{
		Id: &secret.SecretIdentifier{
			Name:         data.Name.ValueString(),
			Organization: organization,
			Domain:       data.Domain.ValueString(),
			Project:      data.Project.ValueString(),
		},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddError("Secret not found", fmt.Sprintf("Secret with name %s not found", data.Name.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to fetch secret", err.Error())
		return
	}

	tflog.Trace(ctx, "GetSecret response", map[string]interface{}{"secret": secretResp})

	data.Id = types.StringValue(secretResp.Secret.Id.Name)
	data.Name = types.StringValue(secretResp.Secret.Id.Name)
	data.Organization = types.StringValue(secretResp.Secret.Id.Organization)

	if secretResp.Secret.Id.Domain != "" {
		data.Domain = types.StringValue(secretResp.Secret.Id.Domain)
	}
	if secretResp.Secret.Id.Project != "" {
		data.Project = types.StringValue(secretResp.Secret.Id.Project)
	}

	if secretResp.Secret.SecretMetadata != nil {
		secretType := strings.ToLower(strings.TrimPrefix(secret.SecretType_name[int32(secretResp.Secret.SecretMetadata.Type)], "SECRET_TYPE_"))
		data.Type = types.StringValue(secretType)

		if secretResp.Secret.SecretMetadata.CreatedTime != nil {
			data.CreatedTime = types.StringValue(secretResp.Secret.SecretMetadata.CreatedTime.AsTime().String())
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
