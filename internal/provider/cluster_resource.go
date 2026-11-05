package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/unionai/cloud/gen/pb-go/cluster"
	"github.com/unionai/cloud/gen/pb-go/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ClusterResource{}
var _ resource.ResourceWithImportState = &ClusterResource{}
var _ resource.ResourceWithValidateConfig = &ClusterResource{}

const clusterStatePrefix = "CLUSTER_STATE_"

// reservedClusterName is the one name the control plane refuses for a cluster: it is the name
// of the reserved default pool and queue.
const reservedClusterName = "default"

func NewClusterResource() resource.Resource {
	return &ClusterResource{}
}

// ClusterResource registers a dataplane cluster with the control plane: the record the
// dataplane's operator heartbeats against. Creating it also creates the co-named queue that
// routes to it.
type ClusterResource struct {
	conn   cluster.ClusterServiceClient
	org    string
	client *providerContext
}

// ClusterResourceModel describes the resource data model.
type ClusterResourceModel struct {
	Id              types.String `tfsdk:"id"`
	Org             types.String `tfsdk:"org"`
	Name            types.String `tfsdk:"name"`
	ClusterPoolName types.String `tfsdk:"cluster_pool_name"`
	State           types.String `tfsdk:"state"`
}

func (r *ClusterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (r *ClusterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Registers a dataplane cluster with the control plane, the record the dataplane's operator " +
			"heartbeats against. Creating a cluster also creates a queue of the same name that routes to it.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier in the form `org/name`.",
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
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Cluster name. Must match the cluster name the dataplane is configured with. Unique within " +
					"the organization, and shared with the queue namespace: it cannot be the name of an existing queue, or `default`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cluster_pool_name": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(defaultClusterPoolName),
				MarkdownDescription: "Cluster pool the cluster belongs to. Defaults to `default`; any other pool must already exist. " +
					"Changing it moves the cluster and its co-named queue, which must be drained first.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Cluster lifecycle state: `active`, `draining`, `drained`, `deleting`, or `deleted`. Managed by the control plane.",
			},
		},
	}
}

func (r *ClusterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.conn = cluster.NewClusterServiceClient(client.conn)
	r.org = client.org
	r.client = client
}

// ValidateConfig rejects the reserved name at plan time rather than at apply.
func (r *ClusterResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ClusterResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Name.ValueString() == reservedClusterName {
		resp.Diagnostics.AddAttributeError(
			path.Root("name"),
			"Reserved cluster name",
			fmt.Sprintf("%q is reserved for the default cluster pool and queue and cannot be used as a cluster name.", reservedClusterName),
		)
	}
}

func (r *ClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Org.IsNull() || data.Org.IsUnknown() {
		data.Org = types.StringValue(r.org)
	}
	org, name := data.Org.ValueString(), data.Name.ValueString()
	resp.Diagnostics.Append(r.client.checkOrgAllowed(org)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createRequest := &cluster.CreateClusterRequest{
		ClusterId:       clusterId(org, name),
		ClusterPoolName: data.ClusterPoolName.ValueString(),
	}
	tflog.Debug(ctx, "CreateCluster request", map[string]interface{}{
		"cluster(create)": createRequest,
	})
	if _, err := r.conn.CreateCluster(ctx, createRequest); err != nil {
		if status.Code(err) == codes.AlreadyExists {
			resp.Diagnostics.AddError(
				"Cluster already exists",
				fmt.Sprintf(
					"A cluster or queue named %q already exists in organization %q. A cluster shares its name with "+
						"the queue that routes to it, so the name must not be taken by either.\n\n"+
						"If it is this cluster (for example one registered with `flyte create cluster`), import it instead:\n\n"+
						"  terraform import unionai_cluster.example %s\n\n"+
						"A soft-deleted cluster keeps its name reserved and cannot be imported until it is undeleted.\n\n%s",
					name, org, name, err,
				),
			)
			return
		}
		resp.Diagnostics.Append(clusterError("create", org, name, err))
		return
	}

	// CreateCluster returns nothing, so read the record back for the computed attributes.
	got, err := r.conn.GetCluster(ctx, &cluster.GetRequest{ClusterId: clusterId(org, name)})
	if err != nil {
		resp.Diagnostics.Append(clusterError("read", org, name, err))
		return
	}
	applyClusterToModel(&data, org, name, got.GetCluster())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	org, name := data.Org.ValueString(), data.Name.ValueString()
	resp.Diagnostics.Append(r.client.checkOrgAllowed(org)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// GetCluster hides soft-deleted clusters, so a deleted cluster reads as gone.
	got, err := r.conn.GetCluster(ctx, &cluster.GetRequest{ClusterId: clusterId(org, name)})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(clusterError("read", org, name, err))
		return
	}
	applyClusterToModel(&data, org, name, got.GetCluster())

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update can only move the cluster to another pool: every other attribute forces replacement.
func (r *ClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, state ClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org, name := data.Org.ValueString(), data.Name.ValueString()
	resp.Diagnostics.Append(r.client.checkOrgAllowed(org)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pool := data.ClusterPoolName.ValueString()
	if pool != state.ClusterPoolName.ValueString() {
		updateRequest := &cluster.UpdateClusterRequest{
			ClusterId:       clusterId(org, name),
			ClusterPoolName: pool,
		}
		tflog.Debug(ctx, "UpdateCluster request", map[string]interface{}{
			"cluster(update)": updateRequest,
		})
		if _, err := r.conn.UpdateCluster(ctx, updateRequest); err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				resp.Diagnostics.AddAttributeError(
					path.Root("cluster_pool_name"),
					"Unable to move cluster to another pool",
					fmt.Sprintf(
						"The control plane refused to move cluster %q from pool %q to %q: %s\n\n"+
							"The cluster's co-named queue moves with it and must be drained first, the target pool must "+
							"exist and be live, and any other live queue that pins the cluster must be unpinned.",
						name, state.ClusterPoolName.ValueString(), pool, err,
					),
				)
				return
			}
			resp.Diagnostics.Append(clusterError("update", org, name, err))
			return
		}
	}

	got, err := r.conn.GetCluster(ctx, &cluster.GetRequest{ClusterId: clusterId(org, name)})
	if err != nil {
		resp.Diagnostics.Append(clusterError("read", org, name, err))
		return
	}
	applyClusterToModel(&data, org, name, got.GetCluster())

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete calls DeleteCluster, which soft-deletes the cluster and its co-named queue. A drained
// cluster is deleted straight away; an active or draining one goes to DELETING and the control
// plane finishes the deletion once its workloads are cleaned up. The name stays reserved.
func (r *ClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ClusterResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	org, name := data.Org.ValueString(), data.Name.ValueString()
	resp.Diagnostics.Append(r.client.checkOrgAllowed(org)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// DeleteCluster needs the generation of the state it is deleting, so read it first.
	got, err := r.conn.GetCluster(ctx, &cluster.GetRequest{ClusterId: clusterId(org, name)})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return
		}
		resp.Diagnostics.Append(clusterError("read", org, name, err))
		return
	}

	drainStatus := got.GetCluster().GetDrainStatus()
	switch drainStatus.GetOverallState() {
	case cluster.ClusterState_CLUSTER_STATE_DELETING, cluster.ClusterState_CLUSTER_STATE_DELETED:
		return
	}

	// Generations start at 1; the server normalizes a never-changed cluster to 1 on read.
	generation := drainStatus.GetGeneration()
	if generation == 0 {
		generation = 1
	}

	deleteRequest := &cluster.DeleteRequest{
		ClusterId:          clusterId(org, name),
		ExpectedGeneration: generation,
	}
	tflog.Debug(ctx, "DeleteCluster request", map[string]interface{}{
		"cluster(delete)": deleteRequest,
	})
	deleted, err := r.conn.DeleteCluster(ctx, deleteRequest)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			return
		case codes.FailedPrecondition:
			resp.Diagnostics.AddError(
				"Unable to delete cluster",
				fmt.Sprintf(
					"The control plane refused to delete cluster %q: %s\n\n"+
						"A live queue other than the cluster's own that pins the cluster blocks deletion; unpin it first. "+
						"If the cluster's state changed while Terraform was deleting it, run destroy again.",
					name, err,
				),
			)
			return
		default:
			resp.Diagnostics.Append(clusterError("delete", org, name, err))
			return
		}
	}

	resp.Diagnostics.AddWarning(
		"Cluster soft-deleted",
		fmt.Sprintf(
			"Cluster %q in organization %q is now %s, along with its co-named queue. Its name stays reserved: "+
				"applying the same configuration again fails with \"already exists\" until the cluster is undeleted "+
				"outside Terraform and imported.",
			name, org, clusterStateToTerraform(deleted.GetCluster().GetDrainStatus().GetOverallState()).ValueString(),
		),
	)
}

// ImportState accepts `org/name`, or a bare cluster name in the provider's organization.
func (r *ClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	org, name := r.org, req.ID
	if parts := strings.Split(req.ID, "/"); len(parts) == 2 {
		org, name = parts[0], parts[1]
	}
	if org == "" || name == "" || strings.Contains(name, "/") {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected `org/name` or `name`, got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(r.client.checkOrgAllowed(org)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), org+"/"+name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("org"), org)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

func clusterId(org, name string) *common.ClusterIdentifier {
	return &common.ClusterIdentifier{Organization: org, Name: name}
}

// clusterStateToTerraform spells the lifecycle state the way queues do. An unset state reads as
// active, as the API documents.
func clusterStateToTerraform(state cluster.ClusterState) types.String {
	if state == cluster.ClusterState_CLUSTER_STATE_UNSPECIFIED {
		state = cluster.ClusterState_CLUSTER_STATE_ACTIVE
	}
	return enumToTerraform(cluster.ClusterState_name[int32(state)], clusterStatePrefix)
}

// applyClusterToModel copies a server-returned cluster back over the model.
func applyClusterToModel(data *ClusterResourceModel, org, name string, got *cluster.Cluster) {
	data.Id = types.StringValue(org + "/" + name)
	data.Org = types.StringValue(org)
	data.Name = types.StringValue(name)
	if pool := got.GetPool().GetName(); pool != "" {
		data.ClusterPoolName = types.StringValue(pool)
	} else if data.ClusterPoolName.IsNull() || data.ClusterPoolName.IsUnknown() {
		data.ClusterPoolName = types.StringValue(defaultClusterPoolName)
	}
	data.State = clusterStateToTerraform(got.GetDrainStatus().GetOverallState())
}

// clusterError turns the API's refusals into errors that say what to do about them.
func clusterError(action, org, name string, err error) diag.Diagnostic {
	switch status.Code(err) {
	case codes.PermissionDenied:
		return diag.NewErrorDiagnostic(
			"Not permitted to manage cluster",
			fmt.Sprintf("The control plane refused to %s cluster %q in organization %q.\n\n%s", action, name, org, err),
		)
	case codes.NotFound:
		return diag.NewErrorDiagnostic(
			"Cluster not found",
			fmt.Sprintf("Unable to %s cluster %q in organization %q: the cluster, or the cluster pool it names, was not found.\n\n%s", action, name, org, err),
		)
	case codes.InvalidArgument:
		return diag.NewErrorDiagnostic(
			"Invalid cluster",
			fmt.Sprintf("The control plane rejected the request to %s cluster %q: %s", action, name, err),
		)
	default:
		return diag.NewErrorDiagnostic(
			"Client Error",
			fmt.Sprintf("Unable to %s cluster %q in organization %q, got error: %s", action, name, org, err),
		)
	}
}
