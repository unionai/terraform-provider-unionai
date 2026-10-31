package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/unionai/cloud/gen/pb-go/cluster"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &NodeGroupsDataSource{}

func NewNodeGroupsDataSource() datasource.DataSource {
	return &NodeGroupsDataSource{}
}

// NodeGroupsDataSource defines the data source implementation.
type NodeGroupsDataSource struct {
	conn   cluster.ManagedClusterServiceClient
	org    string
	client *providerContext
}

// NodeGroupsDataSourceModel describes the data source data model.
type NodeGroupsDataSourceModel struct {
	Id         types.String `tfsdk:"id"`
	Org        types.String `tfsdk:"org"`
	Cluster    types.String `tfsdk:"cluster"`
	Generation types.Int64  `tfsdk:"generation"`
	NodeGroups types.List   `tfsdk:"node_group"`
}

func (d *NodeGroupsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node_groups"
}

func (d *NodeGroupsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	computedString := func(description string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, MarkdownDescription: description}
	}
	computedInt := func(description string) schema.Int64Attribute {
		return schema.Int64Attribute{Computed: true, MarkdownDescription: description}
	}

	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "The node groups declared on a cluster, as stored.",

		Attributes: map[string]schema.Attribute{
			"id": computedString("Identifier in the form `org/cluster`."),
			"org": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization that owns the cluster. Defaults to the provider's organization.",
			},
			"cluster": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Cluster name.",
			},
			"generation": computedInt("The cluster's node group change counter. `0` means the node groups have never been written."),
			"node_group": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The cluster's node groups, sorted by name. See the `unionai_node_groups` resource for what each attribute means.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":                  computedString("Node group name."),
						"min_nodes":             computedInt("Minimum number of nodes."),
						"max_nodes":             computedInt("Maximum number of nodes. `0` means no bound."),
						"node_disk_size_gbs":    computedInt("Disk size of each node, in GB."),
						"node_instance_types":   schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Instance types."},
						"spot_instances":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the group runs on spot instances."},
						"capacity_types":        schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Capacity types."},
						"architecture":          computedString("CPU architecture. Null when unknown."),
						"weight":                computedInt("Preference weight. Null when unset."),
						"max_pods":              computedInt("Pod slots on one node. `0` means unknown."),
						"labels":                schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Kubernetes labels."},
						"limits":                schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Group-wide resource limits."},
						"allocatable_resources": schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Allocatable resources on one node."},
						"taint": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Kubernetes taints.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"key":    computedString("Taint key."),
									"value":  computedString("Taint value."),
									"effect": computedString("Taint effect."),
								},
							},
						},
						"accelerator": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Accelerators on each node.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"device":         computedString("Device name."),
									"manufacturer":   computedString("Manufacturer."),
									"count":          computedInt("Devices per node."),
									"partition_size": computedString("Multi-instance partition size."),
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *NodeGroupsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.conn = cluster.NewManagedClusterServiceClient(client.conn)
	d.org = client.org
	d.client = client
}

func (d *NodeGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NodeGroupsDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Org.IsNull() {
		data.Org = types.StringValue(d.org)
	}
	org, clusterName := data.Org.ValueString(), data.Cluster.ValueString()
	resp.Diagnostics.Append(d.client.checkOrgAllowed(org)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := d.conn.GetNodeGroups(ctx, &cluster.GetNodeGroupsRequest{Id: managedClusterId(org, clusterName)})
	if err != nil {
		resp.Diagnostics.Append(nodeGroupsError("read", org, clusterName, err))
		return
	}

	models, diags := nodeGroupsToModels(ctx, got.GetNodeGroups())
	resp.Diagnostics.Append(diags...)
	data.NodeGroups, diags = types.ListValueFrom(ctx, nodeGroupObjectType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = types.StringValue(org + "/" + clusterName)
	data.Generation = types.Int64Value(int64(got.GetGeneration()))

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
