package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PolicyDataSource{}

func NewPolicyDataSource() datasource.DataSource {
	return &PolicyDataSource{}
}

// PolicyDataSource defines the data source implementation.
type PolicyDataSource struct {
}

type ResourceType string

const (
	ORG     ResourceType = "org"
	DOMAIN  ResourceType = "org/domain"
	PROJECT ResourceType = "org/domain/project"
)

// PolicyDataSourceModel describes the data source data model.
type PolicyDataSourceModel struct {
	Id          types.String                   `tfsdk:"id"`
	Bindings    []PolicyBindingDataSourceModel `tfsdk:"bindings"`
	Description types.String                   `tfsdk:"description"`
}

type PolicyBindingDataSourceModel struct {
	RoleId   types.String `tfsdk:"role_id"`
	Resource types.String `tfsdk:"resource"`
}

func (d *PolicyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (d *PolicyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Policy data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Policy identifier",
				Required:            true,
			},
			"bindings": schema.ListNestedAttribute{
				MarkdownDescription: "Policy bindings",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role_id": schema.StringAttribute{
							MarkdownDescription: "Role identifier",
							Computed:            true,
						},
						"resource": schema.StringAttribute{
							MarkdownDescription: "Resource name",
							Computed:            true,
						},
					},
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Policy description",
				Computed:            true,
			},
		},
	}
}

func (d *PolicyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	_, ok := req.ProviderData.(*providerContext)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *providerContext, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}
}

func (d *PolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PolicyDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := d.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read policy, got error: %s", err))
	//     return
	// }

	// Example: populate from API or static values
	dummy := [][]string{
		{"viewer", "union-internal/production"},
		{"admin", "union-internal/development/flytesnacks"},
	}
	data.Bindings = make([]PolicyBindingDataSourceModel, len(dummy))
	for i, dummy_data := range dummy {
		data.Bindings[i] = PolicyBindingDataSourceModel{
			RoleId:   types.StringValue(dummy_data[0]),
			Resource: types.StringValue(dummy_data[1]),
		}
	}

	data.Description = types.StringValue("")

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "read a data source")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
