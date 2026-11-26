package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/unionai/cloud/gen/pb-go/common"
	"github.com/unionai/cloud/gen/pb-go/fleet"
)

var _ resource.Resource = &ClusterConfigurationResource{}
var _ resource.ResourceWithImportState = &ClusterConfigurationResource{}

func NewClusterConfigurationResource() resource.Resource {
	return &ClusterConfigurationResource{}
}

// ClusterConfigurationResource manages a dataplane cluster's Helm values
// document -- the "config" tab in the cluster settings UI -- through
// FleetService deployment values.
type ClusterConfigurationResource struct {
	conn fleet.FleetServiceClient
	ctx  *providerContext
}

type ClusterConfigurationResourceModel struct {
	Id                types.String `tfsdk:"id"`
	Org               types.String `tfsdk:"org"`
	ClusterName       types.String `tfsdk:"cluster_name"`
	ValuesYaml        yamlDocumentValue `tfsdk:"values_yaml"`
	Revision          types.Int64  `tfsdk:"revision"`
	ValuesFingerprint types.String `tfsdk:"values_fingerprint"`
	ApplyRevision     types.Bool   `tfsdk:"apply_revision"`
	WaitForRollout    types.Bool   `tfsdk:"wait_for_rollout"`
	RolloutTimeout    types.String `tfsdk:"rollout_timeout"`
	State             types.String `tfsdk:"state"`
}

func (r *ClusterConfigurationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_configuration"
}

func (r *ClusterConfigurationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the fleet values (Helm values) document for a dataplane cluster. " +
			"Requires MANAGE_CLUSTER on the cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier: <org>/<cluster_name>.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"org": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization. Defaults to the provider's org.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Dataplane cluster name, e.g. my-cluster.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"values_yaml": schema.StringAttribute{
				CustomType: yamlDocumentType{},
				Required:   true,
				// The document can carry credentials such as secrets.admin.clientSecret.
				Sensitive:           true,
				MarkdownDescription: "Fleet values YAML document for the cluster.",
			},
			"revision": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Server-assigned revision of the values document.",
			},
			"values_fingerprint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-computed fingerprint of the values document.",
			},
			"apply_revision": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
				MarkdownDescription: "Roll the new revision out after writing it. Writing values alone only " +
					"stores a pending revision; the rollout is a separate operation.",
			},
			"wait_for_rollout": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Block until the rolled-out revision reaches APPLIED or FAILED.",
			},
			"rollout_timeout": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("10m"),
				MarkdownDescription: "How long to wait for the rollout, e.g. \"10m\".",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "State of the current revision: PENDING, APPLYING, APPLIED, FAILED or SUPERSEDED.",
			},
		},
	}
}

func (r *ClusterConfigurationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*providerContext)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *providerContext, got: %T.", req.ProviderData),
		)
		return
	}
	r.ctx = client
	r.conn = fleet.NewFleetServiceClient(client.conn)
}

func (r *ClusterConfigurationResource) clusterId(data *ClusterConfigurationResourceModel) *common.ClusterIdentifier {
	org := data.Org.ValueString()
	if org == "" {
		org = r.ctx.org
	}
	return &common.ClusterIdentifier{Organization: org, Name: data.ClusterName.ValueString()}
}

// fetch returns the current fleet-values revision for the cluster.
func (r *ClusterConfigurationResource) fetch(ctx context.Context, id *common.ClusterIdentifier) (*fleet.DeploymentRevision, error) {
	resp, err := r.conn.GetDeploymentValues(ctx, &fleet.GetDeploymentValuesRequest{
		ClusterId: id,
		View:      fleet.ValuesView_VALUES_VIEW_FLEET_VALUES,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetRevision(), nil
}

// write pushes the values document, passing the revision we last read so the
// server can reject a concurrent edit.
func (r *ClusterConfigurationResource) write(ctx context.Context, data *ClusterConfigurationResourceModel) error {
	id := r.clusterId(data)
	current, err := r.fetch(ctx, id)
	if err != nil {
		return err
	}
	// The top-level values_yaml field is documented as the legacy equivalent of
	// change.document{FLEET_VALUES, values_yaml}, but they are not
	// interchangeable in practice: the document variant returns
	// Unimplemented ("change needs stored templates; send values_yaml") against
	// a cluster without stored templates, so send the legacy field.
	updated, err := r.conn.UpdateDeploymentValues(ctx, &fleet.UpdateDeploymentValuesRequest{
		ClusterId:        id,
		ExpectedRevision: current.GetRevision(),
		ValuesYaml:       data.ValuesYaml.ValueString(),
	})
	if err != nil {
		return err
	}
	if !data.ApplyRevision.ValueBool() {
		return nil
	}
	// Storing values only creates a pending revision; ApplyDeployment accepts the
	// latest stored revision and starts the rollout.
	if _, err := r.conn.ApplyDeployment(ctx, &fleet.ApplyDeploymentRequest{ClusterId: id}); err != nil {
		return fmt.Errorf("values stored as revision %d but rollout failed: %w",
			updated.GetRevision().GetRevision(), err)
	}
	if !data.WaitForRollout.ValueBool() {
		return nil
	}
	return r.waitForRollout(ctx, id, updated.GetRevision().GetRevision(), data.RolloutTimeout.ValueString())
}

// waitForRollout polls the revision until it leaves PENDING/APPLYING.
func (r *ClusterConfigurationResource) waitForRollout(ctx context.Context, id *common.ClusterIdentifier, revision uint64, timeout string) error {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid rollout_timeout %q: %w", timeout, err)
	}
	deadline := time.Now().Add(d)
	for {
		resp, err := r.conn.GetDeploymentRevision(ctx, &fleet.GetDeploymentRevisionRequest{
			ClusterId: id, Revision: &revision,
		})
		if err != nil {
			return err
		}
		switch resp.GetRevision().GetState() {
		case fleet.DeploymentRevisionState_DEPLOYMENT_REVISION_STATE_APPLIED:
			return nil
		case fleet.DeploymentRevisionState_DEPLOYMENT_REVISION_STATE_FAILED:
			return fmt.Errorf("rollout of revision %d failed: %s",
				revision, resp.GetRevision().GetLifecycleMessage())
		case fleet.DeploymentRevisionState_DEPLOYMENT_REVISION_STATE_SUPERSEDED:
			return fmt.Errorf("revision %d was superseded before it rolled out", revision)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for revision %d to roll out (state %s)",
				timeout, revision, resp.GetRevision().GetState())
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (r *ClusterConfigurationResource) sync(ctx context.Context, data *ClusterConfigurationResourceModel) error {
	id := r.clusterId(data)
	rev, err := r.fetch(ctx, id)
	if err != nil {
		return err
	}
	data.Org = types.StringValue(id.GetOrganization())
	data.Revision = types.Int64Value(int64(rev.GetRevision()))
	data.ValuesFingerprint = types.StringValue(rev.GetValuesFingerprint())
	data.State = types.StringValue(rev.GetState().String())
	data.Id = types.StringValue(fmt.Sprintf("%s/%s", id.GetOrganization(), id.GetName()))
	return nil
}

func (r *ClusterConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to set cluster configuration: %s", err))
		return
	}
	if err := r.sync(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read back cluster configuration: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterConfigurationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := r.clusterId(&data)
	rev, err := r.fetch(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster configuration: %s", err))
		return
	}
	data.ValuesYaml = newYamlDocumentValue(rev.GetValuesYaml())
	if err := r.sync(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster configuration: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ClusterConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update cluster configuration: %s", err))
		return
	}
	if err := r.sync(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read back cluster configuration: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete only drops the resource from state. A cluster always has a values
// document, and FleetService exposes no RPC to remove one; rolling back to a
// previous revision is a separate, explicit operation.
func (r *ClusterConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Cluster configuration not removed",
		"Removing this resource stops Terraform managing the cluster's values document, "+
			"but leaves the document in place on the cluster.",
	)
}

// ImportState accepts "<org>/<cluster_name>". Both parts are needed because
// Read addresses the cluster by identifier, not by the opaque id.
func (r *ClusterConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	org, cluster, ok := strings.Cut(req.ID, "/")
	if !ok || org == "" || cluster == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"<org>/<cluster_name>\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("org"), org)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_name"), cluster)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
