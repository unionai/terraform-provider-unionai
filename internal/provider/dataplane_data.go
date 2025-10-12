package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/unionai/cloud/gen/pb-go/cluster"
	"github.com/unionai/cloud/gen/pb-go/common"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &DataplaneDataSource{}

func NewDataplaneDataSource() datasource.DataSource {
	return &DataplaneDataSource{}
}

// DataplaneDataSource defines the data source implementation.
type DataplaneDataSource struct {
	conn cluster.ClusterServiceClient
	org  string
}

type DataplaneState string

const (
	DATAPLANE_STATE_ENABLED  DataplaneState = "ENABLED"
	DATAPLANE_STATE_DISABLED DataplaneState = "DISABLED"
	DATAPLANE_STATE_UNKNOWN  DataplaneState = "UNKNOWN"
)

type DataplaneHealth string

const (
	DATAPLANE_HEALTH_HEALTHY   DataplaneHealth = "HEALTHY"
	DATAPLANE_HEALTH_UNHEALTHY DataplaneHealth = "UNHEALTHY"
	DATAPLANE_HEALTH_UNKNOWN   DataplaneHealth = "UNKNOWN"
)

// DataplaneDataSourceModel describes the data source data model.
type DataplaneDataSourceModel struct {
	Id     types.String `tfsdk:"id"`
	State  types.String `tfsdk:"state"`
	Health types.String `tfsdk:"health"`
}

func (d *DataplaneDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataplane"
}

func (d *DataplaneDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Cluster data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cluster identifier",
				Required:            true,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Cluster state",
				Computed:            true,
			},
			"health": schema.StringAttribute{
				MarkdownDescription: "Dataplane health",
				Computed:            true,
			},
		},
	}
}

func (d *DataplaneDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DataplaneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DataplaneDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read cluster
	c, err := d.conn.GetCluster(context.Background(), &cluster.GetRequest{
		ClusterId: &common.ClusterIdentifier{
			Name:         data.Id.ValueString(),
			Organization: d.org,
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch cluster", err.Error())
		return
	}
	tflog.Trace(ctx, "GetCluster response", map[string]interface{}{"cluster": c})

	data.Health = parseHealth(c.Cluster.Status.Health)
	data.State = parseStatus(c.Cluster.Status.State)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func parseHealth(h cluster.Status_Health) types.String {
	switch h {
	case cluster.Status_HEALTHY:
		return types.StringValue(string(DATAPLANE_HEALTH_HEALTHY))
	case cluster.Status_UNHEALTHY:
		return types.StringValue(string(DATAPLANE_HEALTH_UNHEALTHY))
	default:
		return types.StringValue(string(DATAPLANE_HEALTH_UNKNOWN))
	}
}

func parseStatus(s cluster.State) types.String {
	switch s {
	case cluster.State_STATE_ENABLED:
		return types.StringValue(string(DATAPLANE_STATE_ENABLED))
	case cluster.State_STATE_DISABLED:
		return types.StringValue(string(DATAPLANE_STATE_DISABLED))
	default:
		return types.StringValue(string(DATAPLANE_STATE_UNKNOWN))
	}
}
