package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flyteorg/flyte/v2/gen/go/flyteidl2/settings"
)

var _ resource.Resource = &SettingsOverrideResource{}
var _ resource.ResourceWithImportState = &SettingsOverrideResource{}
var _ resource.ResourceWithValidateConfig = &SettingsOverrideResource{}

func NewSettingsOverrideResource() resource.Resource {
	return &SettingsOverrideResource{}
}

// SettingsOverrideResource manages the settings explicitly overridden at one
// scope via flyteidl2 SettingsService. Scope follows the service's own rule and
// is implied by which key fields are set: org only, org+domain, or
// org+domain+project. Attributes left unset inherit from the parent scope.
//
// Distinct from unionai_project_domain_attributes, which writes v1 Admin
// cluster-resource attributes.
type SettingsOverrideResource struct {
	conn settings.SettingsServiceClient
	ctx  *providerContext
}

type SettingsOverrideResourceModel struct {
	Id      types.String `tfsdk:"id"`
	Org     types.String `tfsdk:"org"`
	Domain  types.String `tfsdk:"domain"`
	Project types.String `tfsdk:"project"`
	Scope   types.String `tfsdk:"scope"`

	RunBaseDir     types.String `tfsdk:"run_base_dir"`
	DefaultQueue   types.String `tfsdk:"default_queue"`
	RawDataPath    types.String `tfsdk:"raw_data_path"`
	ServiceAccount types.String `tfsdk:"service_account"`
}

func (r *SettingsOverrideResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_settings_override"
}

// scopeKeys identify the scope level; changing one moves the record to a
// different scope rather than editing it in place.
func scopeKeyAttr(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:            true,
		MarkdownDescription: desc,
		PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
	}
}

func (r *SettingsOverrideResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the settings explicitly overridden at one scope. " +
			"Scope is implied by which fields are set: none (org), `domain`, or `domain` + `project`. " +
			"Attributes left unset inherit from the parent scope.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier: <org>[/<domain>[/<project>]].",
			},
			"org": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization. Defaults to the provider's org.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"domain": scopeKeyAttr("Domain, e.g. development. Omit for org scope."),
			"project": scopeKeyAttr("Project name. Requires `domain`. Omit for org or domain scope."),
			"scope": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resolved scope level: ORG, DOMAIN or PROJECT.",
			},
			"run_base_dir": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Base directory for run metadata such as inputs/outputs (e.g. gs://my-bucket/prefix).",
			},
			"default_queue": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Default queue for task runs.",
			},
			"raw_data_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Base path for raw data storage (e.g. gs://my-bucket/prefix).",
			},
			"service_account": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Kubernetes service account for task pods.",
			},
		},
	}
}

// ValidateConfig rejects a project without a domain: the service infers scope
// from the key, so such a key would silently resolve to org scope.
func (r *SettingsOverrideResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data SettingsOverrideResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.Project.IsNull() && !data.Project.IsUnknown() && data.Domain.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("project"),
			"project requires domain",
			"Settings scope is inferred from the key: a project-scoped override must also set domain.",
		)
	}
}

func (r *SettingsOverrideResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.conn = settings.NewSettingsServiceClient(client.conn)
}

func (r *SettingsOverrideResource) key(data *SettingsOverrideResourceModel) *settings.SettingsKey {
	org := data.Org.ValueString()
	if org == "" {
		org = r.ctx.org
	}
	return &settings.SettingsKey{
		Org:     org,
		Domain:  data.Domain.ValueString(),
		Project: data.Project.ValueString(),
	}
}

func scopeOf(key *settings.SettingsKey) string {
	switch {
	case key.GetProject() != "":
		return "PROJECT"
	case key.GetDomain() != "":
		return "DOMAIN"
	default:
		return "ORG"
	}
}

func idOf(key *settings.SettingsKey) string {
	parts := []string{key.GetOrg()}
	if key.GetDomain() != "" {
		parts = append(parts, key.GetDomain())
	}
	if key.GetProject() != "" {
		parts = append(parts, key.GetProject())
	}
	return strings.Join(parts, "/")
}

func sameKey(a, b *settings.SettingsKey) bool {
	return a.GetOrg() == b.GetOrg() && a.GetDomain() == b.GetDomain() && a.GetProject() == b.GetProject()
}

func str(v types.String) *settings.StringSetting {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return &settings.StringSetting{
		State:       settings.SettingState_SETTING_STATE_VALUE,
		StringValue: v.ValueString(),
	}
}

func readStr(s *settings.StringSetting) types.String {
	if s == nil || s.GetState() != settings.SettingState_SETTING_STATE_VALUE {
		return types.StringNull()
	}
	return types.StringValue(s.GetStringValue())
}

func (r *SettingsOverrideResource) toProto(data *SettingsOverrideResourceModel) *settings.Settings {
	s := &settings.Settings{}
	if v := str(data.RunBaseDir); v != nil {
		s.Run = &settings.RunSettings{RunBaseDir: v}
	}
	if v := str(data.DefaultQueue); v != nil {
		if s.Run == nil {
			s.Run = &settings.RunSettings{}
		}
		s.Run.DefaultQueue = v
	}
	if v := str(data.RawDataPath); v != nil {
		s.Storage = &settings.StorageSettings{RawDataPath: v}
	}
	if v := str(data.ServiceAccount); v != nil {
		s.Security = &settings.SecuritySettings{ServiceAccount: v}
	}
	return s
}

// record returns the stored overrides at exactly this scope, with its version.
// GetSettingsForEdit returns one record per level without applying inheritance,
// so matching the key exactly keeps parent-scope values out of state.
// Version 0 means no record exists at this scope yet.
func (r *SettingsOverrideResource) record(ctx context.Context, key *settings.SettingsKey) (uint64, *settings.Settings, error) {
	resp, err := r.conn.GetSettingsForEdit(ctx, &settings.GetSettingsForEditRequest{Key: key})
	if err != nil {
		return 0, nil, err
	}
	for _, lvl := range resp.GetLevels() {
		if sameKey(lvl.GetKey(), key) {
			return lvl.GetVersion(), lvl.GetSettings(), nil
		}
	}
	return 0, nil, nil
}

func (r *SettingsOverrideResource) upsert(ctx context.Context, data *SettingsOverrideResourceModel) error {
	key := r.key(data)
	version, _, err := r.record(ctx, key)
	if err != nil {
		return err
	}
	payload := r.toProto(data)
	if version == 0 {
		_, err = r.conn.CreateSettings(ctx, &settings.CreateSettingsRequest{Key: key, Settings: payload})
		return err
	}
	_, err = r.conn.UpdateSettings(ctx, &settings.UpdateSettingsRequest{
		Key: key, Settings: payload, Version: version,
	})
	return err
}

func (r *SettingsOverrideResource) stamp(data *SettingsOverrideResourceModel, key *settings.SettingsKey) {
	data.Org = types.StringValue(key.GetOrg())
	data.Scope = types.StringValue(scopeOf(key))
	data.Id = types.StringValue(idOf(key))
}

func (r *SettingsOverrideResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SettingsOverrideResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.upsert(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to set settings override: %s", err))
		return
	}
	r.stamp(&data, r.key(&data))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SettingsOverrideResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SettingsOverrideResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := r.key(&data)
	_, stored, err := r.record(ctx, key)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read settings override: %s", err))
		return
	}
	if stored == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	data.RunBaseDir = readStr(stored.GetRun().GetRunBaseDir())
	data.DefaultQueue = readStr(stored.GetRun().GetDefaultQueue())
	data.RawDataPath = readStr(stored.GetStorage().GetRawDataPath())
	data.ServiceAccount = readStr(stored.GetSecurity().GetServiceAccount())
	r.stamp(&data, key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SettingsOverrideResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SettingsOverrideResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.upsert(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update settings override: %s", err))
		return
	}
	r.stamp(&data, r.key(&data))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete clears every override at this scope so the values inherit from the
// parent again. SettingsService has no delete RPC, so an empty Settings is written.
func (r *SettingsOverrideResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SettingsOverrideResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := r.key(&data)
	version, _, err := r.record(ctx, key)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read settings before delete: %s", err))
		return
	}
	if version == 0 {
		return
	}
	if _, err := r.conn.UpdateSettings(ctx, &settings.UpdateSettingsRequest{
		Key: key, Settings: &settings.Settings{}, Version: version,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to clear settings override: %s", err))
	}
}

// ImportState accepts "<org>", "<org>/<domain>" or "<org>/<domain>/<project>".
// The scope keys must be set explicitly because Read rebuilds the settings key
// from them rather than from the opaque id.
func (r *SettingsOverrideResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) > 3 || parts[0] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"<org>[/<domain>[/<project>]]\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("org"), parts[0])...)
	if len(parts) > 1 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), parts[1])...)
	}
	if len(parts) > 2 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), parts[2])...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
