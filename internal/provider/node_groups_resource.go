package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/unionai/cloud/gen/pb-go/cluster"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &NodeGroupsResource{}
var _ resource.ResourceWithImportState = &NodeGroupsResource{}
var _ resource.ResourceWithValidateConfig = &NodeGroupsResource{}

func NewNodeGroupsResource() resource.Resource {
	return &NodeGroupsResource{}
}

// NodeGroupsResource owns every node group declared on one cluster. SetNodeGroups replaces the
// whole set, so a resource per group could not work: each write would delete the others.
type NodeGroupsResource struct {
	conn cluster.ManagedClusterServiceClient
	org  string
}

// NodeGroupsResourceModel describes the resource data model.
type NodeGroupsResourceModel struct {
	Id         types.String `tfsdk:"id"`
	Org        types.String `tfsdk:"org"`
	Cluster    types.String `tfsdk:"cluster"`
	Generation types.Int64  `tfsdk:"generation"`
	NodeGroups types.Set    `tfsdk:"node_group"`
}

func (r *NodeGroupsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node_groups"
}

func (r *NodeGroupsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "The complete set of node groups declared on a customer-managed cluster. " +
			"Writes replace the whole set: a group left out of the configuration is deleted.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier in the form `org/cluster`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"org": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization that owns the cluster. Defaults to the provider's organization.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cluster": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the cluster whose node groups this resource manages.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"generation": schema.Int64Attribute{
				Computed: true,
				MarkdownDescription: "The cluster's node group change counter, as of the last read or write. Sent back on every " +
					"update so a write is refused if the node groups changed outside Terraform in the meantime.",
			},
		},

		Blocks: map[string]schema.Block{
			"node_group": schema.SetNestedBlock{
				MarkdownDescription: "A node group. Groups are matched by `name`; their order in the configuration does not matter.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Node group name. Must match `^[a-zA-Z][a-zA-Z0-9-]{0,99}$` and be unique within the cluster.",
						},
						"min_nodes": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "Minimum number of nodes. Unset means `0`.",
						},
						"max_nodes": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "Maximum number of nodes. Unset or `0` means the group has no node-count bound.",
						},
						"node_disk_size_gbs": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "Disk size of each node, in GB. At least `1`.",
						},
						"node_instance_types": schema.ListAttribute{
							Required:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Instance types the group may launch, e.g. `[\"m6i.2xlarge\"]`. At least one.",
						},
						"spot_instances": schema.BoolAttribute{
							Optional:            true,
							MarkdownDescription: "Whether the group runs on spot instances. Prefer `capacity_types`, which can also express a fallback.",
						},
						"capacity_types": schema.ListAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Capacity types the group may launch nodes as: `ON_DEMAND`, `SPOT` or `RESERVED`. List several for a group that can fall back from one to another.",
						},
						"architecture": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "CPU architecture of the nodes: `AMD64` or `ARM64`. Unset means unknown, not `AMD64`.",
						},
						"weight": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "Preference weight between `1` and `100`, as Karpenter's `NodePool.spec.weight`. Higher is preferred.",
						},
						"max_pods": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "Pod slots on one node (the kubelet's max-pods). Unset or `0` means unknown.",
						},
						"labels": schema.MapAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Kubernetes labels on the group's nodes.",
						},
						"limits": schema.MapAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Totals the group may not exceed across all of its nodes, as Karpenter's `NodePool.spec.limits`: Kubernetes resource name to quantity, e.g. `{ cpu = \"1000\", \"nvidia.com/gpu\" = \"8\" }`.",
						},
						"allocatable_resources": schema.MapAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Allocatable resources on one node, keyed by `cpu`, `gpu`, `memory`, `storage` or `ephemeral_storage`, e.g. `{ cpu = \"8\", memory = \"32Gi\" }`.",
						},
					},
					Blocks: map[string]schema.Block{
						"taint": schema.ListNestedBlock{
							MarkdownDescription: "A Kubernetes taint on the group's nodes.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"key": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Taint key.",
									},
									"value": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Taint value.",
									},
									"effect": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Taint effect: `NoSchedule`, `PreferNoSchedule` or `NoExecute`.",
									},
								},
							},
						},
						"accelerator": schema.ListNestedBlock{
							MarkdownDescription: "An accelerator type present on each node of the group.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"device": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The cloud provider's name for the device, e.g. `A100` or `nvidia-tesla-t4`.",
									},
									"manufacturer": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The device manufacturer, e.g. `NVIDIA` or `AMD`.",
									},
									"count": schema.Int64Attribute{
										Required:            true,
										MarkdownDescription: "Number of these devices on one node. Greater than `0`.",
									},
									"partition_size": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Multi-instance partition size, e.g. `1g.5gb`. Empty means the device is not partitioned.",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *NodeGroupsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*providerContext)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *providerContext, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.conn = cluster.NewManagedClusterServiceClient(client.conn)
	r.org = client.org
}

// ValidateConfig checks each group against the API's validation rules, and that names are
// unique, so a bad configuration fails at plan time.
func (r *NodeGroupsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data NodeGroupsResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.NodeGroups.IsNull() || data.NodeGroups.IsUnknown() {
		return
	}

	var groups []nodeGroupModel
	// A group built from unknown values (a dynamic block, say) cannot be decoded yet; the
	// server validates it at apply instead.
	if diags := data.NodeGroups.ElementsAs(ctx, &groups, false); diags.HasError() {
		return
	}

	seen := map[string]bool{}
	for _, group := range groups {
		resp.Diagnostics.Append(validateNodeGroup(ctx, group)...)
		if !known(group.Name) {
			continue
		}
		name := group.Name.ValueString()
		if seen[name] {
			resp.Diagnostics.AddAttributeError(
				path.Root("node_group"),
				"Duplicate node group name",
				fmt.Sprintf("Node group %q is declared more than once. Names must be unique within a cluster.", name),
			)
		}
		seen[name] = true
	}
}

func (r *NodeGroupsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NodeGroupsResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Org.IsNull() || data.Org.IsUnknown() {
		data.Org = types.StringValue(r.org)
	}
	org, clusterName := data.Org.ValueString(), data.Cluster.ValueString()

	// Read first so the write is checked against what is there, and so the user hears which
	// existing groups this replaces.
	existing, err := r.conn.GetNodeGroups(ctx, &cluster.GetNodeGroupsRequest{Id: managedClusterId(org, clusterName)})
	if err != nil {
		resp.Diagnostics.Append(nodeGroupsError("read", org, clusterName, err))
		return
	}

	groups, diags := nodeGroupsFromSet(ctx, data.NodeGroups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	generation, err := r.setNodeGroups(ctx, org, clusterName, groups, existing.GetGeneration())
	if err != nil {
		resp.Diagnostics.Append(nodeGroupsError("set", org, clusterName, err))
		return
	}

	if dropped := droppedGroupNames(existing.GetNodeGroups(), groups); len(dropped) > 0 {
		resp.Diagnostics.AddWarning(
			"Existing node groups replaced",
			fmt.Sprintf("Cluster %q already declared node groups that this configuration does not: %s. They have been deleted.",
				clusterName, strings.Join(dropped, ", ")),
		)
	}

	data.Id = types.StringValue(org + "/" + clusterName)
	data.Generation = types.Int64Value(int64(generation))

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NodeGroupsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NodeGroupsResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	org, clusterName := data.Org.ValueString(), data.Cluster.ValueString()
	got, err := r.conn.GetNodeGroups(ctx, &cluster.GetNodeGroupsRequest{Id: managedClusterId(org, clusterName)})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(nodeGroupsError("read", org, clusterName, err))
		return
	}

	priorByName := map[string]nodeGroupModel{}
	if !data.NodeGroups.IsNull() && !data.NodeGroups.IsUnknown() {
		var prior []nodeGroupModel
		resp.Diagnostics.Append(data.NodeGroups.ElementsAs(ctx, &prior, false)...)
		for _, group := range prior {
			priorByName[group.Name.ValueString()] = group
		}
	}

	models, diags := nodeGroupsToModels(ctx, got.GetNodeGroups())
	resp.Diagnostics.Append(diags...)
	for i, model := range models {
		models[i] = fitNodeGroupToPrior(model, priorByName[model.Name.ValueString()])
	}
	data.NodeGroups, diags = types.SetValueFrom(ctx, nodeGroupObjectType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Generation = types.Int64Value(int64(got.GetGeneration()))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NodeGroupsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, state NodeGroupsResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	org, clusterName := data.Org.ValueString(), data.Cluster.ValueString()

	groups, diags := nodeGroupsFromSet(ctx, data.NodeGroups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	generation, err := r.setNodeGroups(ctx, org, clusterName, groups, uint64(state.Generation.ValueInt64()))
	if err != nil {
		resp.Diagnostics.Append(nodeGroupsError("set", org, clusterName, err))
		return
	}

	data.Generation = types.Int64Value(int64(generation))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete writes an empty set, which the API takes as "this cluster declares no node groups".
func (r *NodeGroupsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NodeGroupsResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	org, clusterName := data.Org.ValueString(), data.Cluster.ValueString()
	_, err := r.setNodeGroups(ctx, org, clusterName, nil, uint64(data.Generation.ValueInt64()))
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return
		}
		resp.Diagnostics.Append(nodeGroupsError("delete", org, clusterName, err))
	}
}

// ImportState accepts `org/cluster`, or a bare cluster name in the provider's organization.
func (r *NodeGroupsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	org, clusterName := r.org, req.ID
	if parts := strings.Split(req.ID, "/"); len(parts) == 2 {
		org, clusterName = parts[0], parts[1]
	}
	if org == "" || clusterName == "" || strings.Contains(clusterName, "/") {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected `org/cluster` or `cluster`, got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), org+"/"+clusterName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("org"), org)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster"), clusterName)...)
}

// setNodeGroups writes the whole set. A generation of 0 asks for no concurrency check, which is
// also what the API does with it.
func (r *NodeGroupsResource) setNodeGroups(ctx context.Context, org, clusterName string, groups []*cluster.NodeGroup, expectedGeneration uint64) (uint64, error) {
	request := &cluster.SetNodeGroupsRequest{
		Id:         managedClusterId(org, clusterName),
		NodeGroups: groups,
	}
	if expectedGeneration != 0 {
		request.ExpectedGeneration = &expectedGeneration
	}

	tflog.Debug(ctx, "SetNodeGroups request", map[string]interface{}{
		"node_groups(set)": request,
	})
	set, err := r.conn.SetNodeGroups(ctx, request)
	if err != nil {
		return 0, err
	}
	return set.GetGeneration(), nil
}

func nodeGroupsFromSet(ctx context.Context, set types.Set) ([]*cluster.NodeGroup, diag.Diagnostics) {
	var models []nodeGroupModel
	diags := set.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}

	// Sorted so the request is deterministic; set order carries no meaning.
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name.ValueString() < models[j].Name.ValueString()
	})

	groups := make([]*cluster.NodeGroup, 0, len(models))
	for _, model := range models {
		group, d := nodeGroupFromModel(ctx, model)
		diags.Append(d...)
		groups = append(groups, group)
	}
	return groups, diags
}

func droppedGroupNames(existing, planned []*cluster.NodeGroup) []string {
	keep := map[string]bool{}
	for _, group := range planned {
		keep[group.GetName()] = true
	}
	var dropped []string
	for _, group := range existing {
		if !keep[group.GetName()] {
			dropped = append(dropped, group.GetName())
		}
	}
	sort.Strings(dropped)
	return dropped
}

// nodeGroupsError turns the API's refusals into errors that say what to do about them.
func nodeGroupsError(action, org, clusterName string, err error) diag.Diagnostic {
	switch status.Code(err) {
	case codes.Aborted:
		return diag.NewErrorDiagnostic(
			"Node groups changed outside Terraform",
			fmt.Sprintf("The node groups on cluster %q changed since Terraform last read them, so the write was refused "+
				"rather than overwrite that change. Run `terraform plan` (or `terraform apply -refresh-only`) to see the "+
				"current groups, then apply again.\n\n%s", clusterName, err),
		)
	case codes.PermissionDenied:
		return diag.NewErrorDiagnostic(
			"Not permitted to manage node groups",
			fmt.Sprintf("The control plane refused to %s node groups on cluster %q in organization %q. Only customer-managed "+
				"clusters accept node group writes from their organization, and the caller needs MANAGE_CLUSTER on the cluster. "+
				"Node groups on Union-provisioned (BYOC) clusters are managed by Union.\n\n%s", action, clusterName, org, err),
		)
	case codes.NotFound:
		return diag.NewErrorDiagnostic(
			"Cluster not found",
			fmt.Sprintf("Cluster %q was not found in organization %q.\n\n%s", clusterName, org, err),
		)
	case codes.InvalidArgument:
		return diag.NewErrorDiagnostic(
			"Invalid node groups",
			fmt.Sprintf("The control plane rejected the node groups for cluster %q: %s", clusterName, err),
		)
	default:
		return diag.NewErrorDiagnostic(
			"Client Error",
			fmt.Sprintf("Unable to %s node groups for cluster %q, got error: %s", action, clusterName, err),
		)
	}
}
