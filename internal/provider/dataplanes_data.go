package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/unionai/terraform-provider-unionai/proto/cluster"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &DataplanesDataSource{}

func NewDataplanesDataSource() datasource.DataSource {
	return &DataplanesDataSource{}
}

// DataplanesDataSource defines the data source implementation.
type DataplanesDataSource struct {
	conn cluster.ClusterServiceClient
	org  string
}

// DataplanesDataSourceModel describes the data source data model.
type DataplanesDataSourceModel struct {
	Ids types.Set `tfsdk:"ids"`
}

func (d *DataplanesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataplanes"
}

func (d *DataplanesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Dataplanes data source",

		Attributes: map[string]schema.Attribute{
			"ids": schema.SetAttribute{
				MarkdownDescription: "List of dataplane IDs",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *DataplanesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.conn = cluster.NewClusterServiceClient(client.conn)
	if d.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *cluster.ClusterServiceClient, got: %T. Please report this issue to the provider developers.", d.conn),
		)
		return
	}
	d.org = client.org
}

func (d *DataplanesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DataplanesDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	clusters, err := d.conn.ListClusters(ctx, &cluster.ListRequest{
		Organization: d.org,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch clusters", err.Error())
		return
	}

	// Build a list of dataplane IDs only
	idValues := make([]attr.Value, 0, len(clusters.Clusters))
	for _, c := range clusters.Clusters {
		idValues = append(idValues, types.StringValue(c.Spec.Id.Name))
	}
	data.Ids = types.SetValueMust(types.StringType, idValues)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
