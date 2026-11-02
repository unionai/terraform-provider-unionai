package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/flyteorg/flyte/flyteidl/gen/pb-go/flyteidl/core"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/unionai/cloud/gen/pb-go/cluster"
	"github.com/unionai/cloud/gen/pb-go/common"
)

// nodeGroupNamePattern mirrors the buf.validate rule on NodeGroup.name, so a bad name fails at
// plan time rather than at apply.
var nodeGroupNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]{0,99}$`)

const (
	capacityTypePrefix = "CAPACITY_TYPE_"
	architecturePrefix = "NODE_ARCHITECTURE_"
)

// Taint effects are spelled as Kubernetes spells them, since that is what users copy from.
var taintEffectNames = map[cluster.TaintEffect]string{
	cluster.TaintEffect_NO_SCHEDULE:        "NoSchedule",
	cluster.TaintEffect_PREFER_NO_SCHEDULE: "PreferNoSchedule",
	cluster.TaintEffect_NO_EXECUTE:         "NoExecute",
}

type nodeGroupModel struct {
	Name                 types.String       `tfsdk:"name"`
	MinNodes             types.Int64        `tfsdk:"min_nodes"`
	MaxNodes             types.Int64        `tfsdk:"max_nodes"`
	NodeDiskSizeGbs      types.Int64        `tfsdk:"node_disk_size_gbs"`
	NodeInstanceTypes    types.List         `tfsdk:"node_instance_types"`
	SpotInstances        types.Bool         `tfsdk:"spot_instances"`
	CapacityTypes        types.List         `tfsdk:"capacity_types"`
	Architecture         types.String       `tfsdk:"architecture"`
	Weight               types.Int64        `tfsdk:"weight"`
	MaxPods              types.Int64        `tfsdk:"max_pods"`
	Labels               types.Map          `tfsdk:"labels"`
	Limits               types.Map          `tfsdk:"limits"`
	AllocatableResources types.Map          `tfsdk:"allocatable_resources"`
	Taints               []taintModel       `tfsdk:"taint"`
	Accelerators         []acceleratorModel `tfsdk:"accelerator"`
}

type taintModel struct {
	Key    types.String `tfsdk:"key"`
	Value  types.String `tfsdk:"value"`
	Effect types.String `tfsdk:"effect"`
}

type acceleratorModel struct {
	Device        types.String `tfsdk:"device"`
	Manufacturer  types.String `tfsdk:"manufacturer"`
	Count         types.Int64  `tfsdk:"count"`
	PartitionSize types.String `tfsdk:"partition_size"`
}

var taintAttrTypes = map[string]attr.Type{
	"key":    types.StringType,
	"value":  types.StringType,
	"effect": types.StringType,
}

var acceleratorAttrTypes = map[string]attr.Type{
	"device":         types.StringType,
	"manufacturer":   types.StringType,
	"count":          types.Int64Type,
	"partition_size": types.StringType,
}

var nodeGroupAttrTypes = map[string]attr.Type{
	"name":                  types.StringType,
	"min_nodes":             types.Int64Type,
	"max_nodes":             types.Int64Type,
	"node_disk_size_gbs":    types.Int64Type,
	"node_instance_types":   types.ListType{ElemType: types.StringType},
	"spot_instances":        types.BoolType,
	"capacity_types":        types.ListType{ElemType: types.StringType},
	"architecture":          types.StringType,
	"weight":                types.Int64Type,
	"max_pods":              types.Int64Type,
	"labels":                types.MapType{ElemType: types.StringType},
	"limits":                types.MapType{ElemType: types.StringType},
	"allocatable_resources": types.MapType{ElemType: types.StringType},
	"taint":                 types.ListType{ElemType: types.ObjectType{AttrTypes: taintAttrTypes}},
	"accelerator":           types.ListType{ElemType: types.ObjectType{AttrTypes: acceleratorAttrTypes}},
}

var nodeGroupObjectType = types.ObjectType{AttrTypes: nodeGroupAttrTypes}

func managedClusterId(org, name string) *common.ManagedClusterIdentifier {
	return &common.ManagedClusterIdentifier{
		Org:  &common.OrgIdentifier{Name: org},
		Name: name,
	}
}

func capacityTypeFromString(text string) (cluster.CapacityType, bool) {
	value, ok := cluster.CapacityType_value[capacityTypePrefix+text]
	if !ok || value == int32(cluster.CapacityType_CAPACITY_TYPE_UNSPECIFIED) {
		return cluster.CapacityType_CAPACITY_TYPE_UNSPECIFIED, false
	}
	return cluster.CapacityType(value), true
}

// capacityTypeToString is false for a value this provider has no name for, such as an enum
// value added to the API after this provider was built.
func capacityTypeToString(capacityType cluster.CapacityType) (string, bool) {
	name, ok := cluster.CapacityType_name[int32(capacityType)]
	if !ok || capacityType == cluster.CapacityType_CAPACITY_TYPE_UNSPECIFIED {
		return "", false
	}
	return strings.TrimPrefix(name, capacityTypePrefix), true
}

func architectureFromString(text string) (cluster.NodeArchitecture, bool) {
	value, ok := cluster.NodeArchitecture_value[architecturePrefix+text]
	if !ok || value == int32(cluster.NodeArchitecture_NODE_ARCHITECTURE_UNSPECIFIED) {
		return cluster.NodeArchitecture_NODE_ARCHITECTURE_UNSPECIFIED, false
	}
	return cluster.NodeArchitecture(value), true
}

// architectureToString is false for a value this provider has no name for. Unspecified is
// represented as null by the caller, so it is false too.
func architectureToString(architecture cluster.NodeArchitecture) (string, bool) {
	name, ok := cluster.NodeArchitecture_name[int32(architecture)]
	if !ok || architecture == cluster.NodeArchitecture_NODE_ARCHITECTURE_UNSPECIFIED {
		return "", false
	}
	return strings.TrimPrefix(name, architecturePrefix), true
}

func taintEffectFromString(text string) (cluster.TaintEffect, bool) {
	for effect, name := range taintEffectNames {
		if name == text {
			return effect, true
		}
	}
	return cluster.TaintEffect_UNSPECIFIED, false
}

// taintEffectToString is false for an effect this provider has no name for.
func taintEffectToString(effect cluster.TaintEffect) (string, bool) {
	name, ok := taintEffectNames[effect]
	return name, ok
}

// Allocatable resource keys are the flyteidl ResourceName values in lower case: cpu, gpu,
// memory, storage, ephemeral_storage.
func resourceNameFromString(text string) (core.Resources_ResourceName, bool) {
	value, ok := core.Resources_ResourceName_value[strings.ToUpper(text)]
	if !ok || value == int32(core.Resources_UNKNOWN) || text != strings.ToLower(text) {
		return core.Resources_UNKNOWN, false
	}
	return core.Resources_ResourceName(value), true
}

func resourceNameToString(resourceName core.Resources_ResourceName) string {
	return strings.ToLower(resourceName.String())
}

func enumNames(prefix string, values map[string]int32) []string {
	var names []string
	for name, value := range values {
		if value != 0 {
			names = append(names, strings.TrimPrefix(name, prefix))
		}
	}
	sort.Strings(names)
	return names
}

func validCapacityTypes() []string { return enumNames(capacityTypePrefix, cluster.CapacityType_value) }
func validArchitectures() []string {
	return enumNames(architecturePrefix, cluster.NodeArchitecture_value)
}
func validResourceNames() []string {
	var names []string
	for _, name := range enumNames("", core.Resources_ResourceName_value) {
		names = append(names, strings.ToLower(name))
	}
	return names
}
func validTaintEffects() []string {
	var names []string
	for _, name := range taintEffectNames {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// nodeGroupToModel converts a stored node group into its Terraform shape. Collections come back
// empty rather than null, matching the schema defaults, so a group that never set them reads
// back without a diff. An enum value this provider has no name for is an error rather than a
// state value the provider would refuse to write back.
func nodeGroupToModel(ctx context.Context, group *cluster.NodeGroup) (nodeGroupModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	unsupported := func(what string, value fmt.Stringer) {
		diags.AddError("Unsupported node group "+what,
			fmt.Sprintf("Node group %q has %s %s, which this version of the provider does not know. Upgrade the provider.",
				group.GetName(), what, value))
	}

	capacityTypes := make([]string, 0, len(group.GetCapacityTypes()))
	for _, capacityType := range group.GetCapacityTypes() {
		name, ok := capacityTypeToString(capacityType)
		if !ok {
			unsupported("capacity type", capacityType)
			continue
		}
		capacityTypes = append(capacityTypes, name)
	}

	allocatable := make(map[string]string, len(group.GetAllocatableResources()))
	for _, entry := range group.GetAllocatableResources() {
		allocatable[resourceNameToString(entry.GetName())] = entry.GetValue()
	}

	labels := group.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	limits := group.GetLimits()
	if limits == nil {
		limits = map[string]string{}
	}
	instanceTypes := group.GetNodeInstanceTypes()
	if instanceTypes == nil {
		instanceTypes = []string{}
	}

	model := nodeGroupModel{
		Name:            types.StringValue(group.GetName()),
		MinNodes:        types.Int64Value(int64(group.GetMinNodes())),
		MaxNodes:        types.Int64Value(int64(group.GetMaxNodes())),
		NodeDiskSizeGbs: types.Int64Value(int64(group.GetNodeDiskSizeGbs())),
		SpotInstances:   types.BoolValue(group.GetSpotInstances()),
		Architecture:    types.StringNull(),
		Weight:          types.Int64Null(),
		MaxPods:         types.Int64Value(int64(group.GetMaxPods())),
		Taints:          make([]taintModel, 0, len(group.GetTaints())),
		Accelerators:    make([]acceleratorModel, 0, len(group.GetAccelerators())),
	}
	if group.GetArchitecture() != cluster.NodeArchitecture_NODE_ARCHITECTURE_UNSPECIFIED {
		name, ok := architectureToString(group.GetArchitecture())
		if !ok {
			unsupported("architecture", group.GetArchitecture())
		}
		model.Architecture = types.StringValue(name)
	}
	if group.Weight != nil {
		model.Weight = types.Int64Value(int64(group.GetWeight()))
	}

	var convertDiags diag.Diagnostics
	model.NodeInstanceTypes, convertDiags = types.ListValueFrom(ctx, types.StringType, instanceTypes)
	diags.Append(convertDiags...)
	model.CapacityTypes, convertDiags = types.ListValueFrom(ctx, types.StringType, capacityTypes)
	diags.Append(convertDiags...)
	model.Labels, convertDiags = types.MapValueFrom(ctx, types.StringType, labels)
	diags.Append(convertDiags...)
	model.Limits, convertDiags = types.MapValueFrom(ctx, types.StringType, limits)
	diags.Append(convertDiags...)
	model.AllocatableResources, convertDiags = types.MapValueFrom(ctx, types.StringType, allocatable)
	diags.Append(convertDiags...)

	for _, taint := range group.GetTaints() {
		effect, ok := taintEffectToString(taint.GetEffect())
		if !ok {
			unsupported("taint effect", taint.GetEffect())
		}
		model.Taints = append(model.Taints, taintModel{
			Key:    types.StringValue(taint.GetKey()),
			Value:  types.StringValue(taint.GetValue()),
			Effect: types.StringValue(effect),
		})
	}
	for _, accelerator := range group.GetAccelerators() {
		model.Accelerators = append(model.Accelerators, acceleratorModel{
			Device:        types.StringValue(accelerator.GetDevice()),
			Manufacturer:  types.StringValue(accelerator.GetManufacturer()),
			Count:         types.Int64Value(accelerator.GetCount()),
			PartitionSize: types.StringValue(accelerator.GetPartitionSize()),
		})
	}

	return model, diags
}

// nodeGroupsToModels converts and sorts by name, so the data source lists groups in a stable
// order regardless of how they were stored.
func nodeGroupsToModels(ctx context.Context, groups []*cluster.NodeGroup) ([]nodeGroupModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	models := make([]nodeGroupModel, 0, len(groups))
	for _, group := range groups {
		model, d := nodeGroupToModel(ctx, group)
		diags.Append(d...)
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name.ValueString() < models[j].Name.ValueString()
	})
	return models, diags
}

// fitNodeGroupToPrior shapes a group read from the server like the one already in state. The API
// cannot tell an unset field from a zero one, so a zero value is written as null unless prior had
// it set; otherwise `min_nodes = 0` in config and an omitted min_nodes would each diff against
// the other's read. A group with no prior (an import, or drift) gets nulls throughout.
func fitNodeGroupToPrior(model nodeGroupModel, prior nodeGroupModel) nodeGroupModel {
	nullIfZero := func(value types.Int64, priorValue types.Int64) types.Int64 {
		if value.ValueInt64() == 0 && priorValue.IsNull() {
			return types.Int64Null()
		}
		return value
	}
	model.MinNodes = nullIfZero(model.MinNodes, prior.MinNodes)
	model.MaxNodes = nullIfZero(model.MaxNodes, prior.MaxNodes)
	model.MaxPods = nullIfZero(model.MaxPods, prior.MaxPods)

	if !model.SpotInstances.ValueBool() && prior.SpotInstances.IsNull() {
		model.SpotInstances = types.BoolNull()
	}
	if len(model.CapacityTypes.Elements()) == 0 && prior.CapacityTypes.IsNull() {
		model.CapacityTypes = types.ListNull(types.StringType)
	}
	if len(model.Labels.Elements()) == 0 && prior.Labels.IsNull() {
		model.Labels = types.MapNull(types.StringType)
	}
	if len(model.Limits.Elements()) == 0 && prior.Limits.IsNull() {
		model.Limits = types.MapNull(types.StringType)
	}
	if len(model.AllocatableResources.Elements()) == 0 && prior.AllocatableResources.IsNull() {
		model.AllocatableResources = types.MapNull(types.StringType)
	}

	// Copied so the caller's slices are left alone.
	model.Taints = append([]taintModel{}, model.Taints...)
	model.Accelerators = append([]acceleratorModel{}, model.Accelerators...)
	for i := range model.Taints {
		if model.Taints[i].Value.ValueString() == "" && (i >= len(prior.Taints) || prior.Taints[i].Value.IsNull()) {
			model.Taints[i].Value = types.StringNull()
		}
	}
	for i := range model.Accelerators {
		if model.Accelerators[i].PartitionSize.ValueString() == "" && (i >= len(prior.Accelerators) || prior.Accelerators[i].PartitionSize.IsNull()) {
			model.Accelerators[i].PartitionSize = types.StringNull()
		}
	}
	return model
}

// nodeGroupFromModel builds the proto for one group. It assumes validateNodeGroup has already
// passed, but still refuses values it cannot represent rather than sending a zero in their place.
func nodeGroupFromModel(ctx context.Context, model nodeGroupModel) (*cluster.NodeGroup, diag.Diagnostics) {
	var diags diag.Diagnostics
	name := model.Name.ValueString()

	group := &cluster.NodeGroup{
		Name:            name,
		MinNodes:        uint32(model.MinNodes.ValueInt64()),
		MaxNodes:        uint32(model.MaxNodes.ValueInt64()),
		NodeDiskSizeGbs: uint32(model.NodeDiskSizeGbs.ValueInt64()),
		SpotInstances:   model.SpotInstances.ValueBool(),
		MaxPods:         uint32(model.MaxPods.ValueInt64()),
	}

	diags.Append(model.NodeInstanceTypes.ElementsAs(ctx, &group.NodeInstanceTypes, false)...)
	if !model.Labels.IsNull() {
		diags.Append(model.Labels.ElementsAs(ctx, &group.Labels, false)...)
	}
	if !model.Limits.IsNull() {
		diags.Append(model.Limits.ElementsAs(ctx, &group.Limits, false)...)
	}

	if !model.Architecture.IsNull() {
		architecture, ok := architectureFromString(model.Architecture.ValueString())
		if !ok {
			diags.AddError("Invalid architecture", fmt.Sprintf("Node group %q: unknown architecture %q.", name, model.Architecture.ValueString()))
		}
		group.Architecture = architecture
	}
	if !model.Weight.IsNull() {
		weight := int32(model.Weight.ValueInt64())
		group.Weight = &weight
	}

	var capacityTypes []string
	if !model.CapacityTypes.IsNull() {
		diags.Append(model.CapacityTypes.ElementsAs(ctx, &capacityTypes, false)...)
	}
	for _, text := range capacityTypes {
		capacityType, ok := capacityTypeFromString(text)
		if !ok {
			diags.AddError("Invalid capacity type", fmt.Sprintf("Node group %q: unknown capacity type %q.", name, text))
			continue
		}
		group.CapacityTypes = append(group.CapacityTypes, capacityType)
	}

	var allocatable map[string]string
	if !model.AllocatableResources.IsNull() {
		diags.Append(model.AllocatableResources.ElementsAs(ctx, &allocatable, false)...)
	}
	// Sorted so the request is deterministic; the stored order carries no meaning.
	resourceKeys := make([]string, 0, len(allocatable))
	for key := range allocatable {
		resourceKeys = append(resourceKeys, key)
	}
	sort.Strings(resourceKeys)
	for _, key := range resourceKeys {
		resourceName, ok := resourceNameFromString(key)
		if !ok {
			diags.AddError("Invalid allocatable resource", fmt.Sprintf("Node group %q: unknown resource %q.", name, key))
			continue
		}
		group.AllocatableResources = append(group.AllocatableResources, &core.Resources_ResourceEntry{
			Name:  resourceName,
			Value: allocatable[key],
		})
	}

	for _, taint := range model.Taints {
		effect, ok := taintEffectFromString(taint.Effect.ValueString())
		if !ok {
			diags.AddError("Invalid taint effect", fmt.Sprintf("Node group %q: unknown taint effect %q.", name, taint.Effect.ValueString()))
			continue
		}
		group.Taints = append(group.Taints, &cluster.Taint{
			Key:    taint.Key.ValueString(),
			Value:  taint.Value.ValueString(),
			Effect: effect,
		})
	}

	for _, accelerator := range model.Accelerators {
		group.Accelerators = append(group.Accelerators, &cluster.NodeGroupAccelerator{
			Device:        accelerator.Device.ValueString(),
			Manufacturer:  accelerator.Manufacturer.ValueString(),
			Count:         accelerator.Count.ValueInt64(),
			PartitionSize: accelerator.PartitionSize.ValueString(),
		})
	}

	return group, diags
}

// validateNodeGroups checks each group, and that names are unique within the cluster. Unknown
// names are skipped.
func validateNodeGroups(ctx context.Context, models []nodeGroupModel) diag.Diagnostics {
	var diags diag.Diagnostics
	seen := map[string]bool{}
	for _, model := range models {
		diags.Append(validateNodeGroup(ctx, model)...)
		if !known(model.Name) {
			continue
		}
		name := model.Name.ValueString()
		if seen[name] {
			diags.AddAttributeError(
				path.Root("node_group"),
				"Duplicate node group name",
				fmt.Sprintf("Node group %q is declared more than once. Names must be unique within a cluster.", name),
			)
		}
		seen[name] = true
	}
	return diags
}

// validateNodeGroup mirrors the buf.validate rules on NodeGroup, plus a couple of checks the
// server cannot make, so mistakes surface at plan time. Unknown values are skipped; the server
// checks them at apply.
func validateNodeGroup(ctx context.Context, model nodeGroupModel) diag.Diagnostics {
	var diags diag.Diagnostics
	name := model.Name.ValueString()

	if !model.Name.IsUnknown() && !nodeGroupNamePattern.MatchString(name) {
		diags.AddError("Invalid node group name",
			fmt.Sprintf("Node group name %q must start with a letter and contain only letters, digits and hyphens, at most 100 characters.", name))
	}

	checkUint32 := func(attribute string, value types.Int64, minimum int64) {
		if _, problem := int64ToUint32(attribute, value, minimum); problem != "" {
			diags.AddError("Invalid node group "+attribute, fmt.Sprintf("Node group %q: %s", name, problem))
		}
	}
	checkUint32("min_nodes", model.MinNodes, 0)
	checkUint32("max_nodes", model.MaxNodes, 0)
	checkUint32("node_disk_size_gbs", model.NodeDiskSizeGbs, 1)
	checkUint32("max_pods", model.MaxPods, 0)

	if known(model.MinNodes) && known(model.MaxNodes) && model.MaxNodes.ValueInt64() != 0 &&
		model.MinNodes.ValueInt64() > model.MaxNodes.ValueInt64() {
		diags.AddError("Invalid node group bounds",
			fmt.Sprintf("Node group %q: min_nodes (%d) is greater than max_nodes (%d).", name, model.MinNodes.ValueInt64(), model.MaxNodes.ValueInt64()))
	}

	if known(model.Weight) && (model.Weight.ValueInt64() < 1 || model.Weight.ValueInt64() > 100) {
		diags.AddError("Invalid node group weight",
			fmt.Sprintf("Node group %q: weight must be between 1 and 100, got %d.", name, model.Weight.ValueInt64()))
	}

	if !model.NodeInstanceTypes.IsNull() && !model.NodeInstanceTypes.IsUnknown() && len(model.NodeInstanceTypes.Elements()) == 0 {
		diags.AddError("Invalid node group instance types",
			fmt.Sprintf("Node group %q: node_instance_types must list at least one instance type.", name))
	}

	if known(model.Architecture) {
		if _, ok := architectureFromString(model.Architecture.ValueString()); !ok {
			diags.AddError("Invalid node group architecture",
				fmt.Sprintf("Node group %q: architecture must be one of %s, got %q.", name, strings.Join(validArchitectures(), ", "), model.Architecture.ValueString()))
		}
	}

	if !model.CapacityTypes.IsNull() && !model.CapacityTypes.IsUnknown() {
		for _, element := range model.CapacityTypes.Elements() {
			value, ok := element.(types.String)
			if !ok || !known(value) {
				continue
			}
			if _, ok := capacityTypeFromString(value.ValueString()); !ok {
				diags.AddError("Invalid node group capacity type",
					fmt.Sprintf("Node group %q: capacity_types entries must be one of %s, got %q.", name, strings.Join(validCapacityTypes(), ", "), value.ValueString()))
			}
		}
	}

	if !model.AllocatableResources.IsNull() && !model.AllocatableResources.IsUnknown() {
		for key := range model.AllocatableResources.Elements() {
			if _, ok := resourceNameFromString(key); !ok {
				diags.AddError("Invalid node group allocatable resource",
					fmt.Sprintf("Node group %q: allocatable_resources keys must be one of %s, got %q.", name, strings.Join(validResourceNames(), ", "), key))
			}
		}
	}

	if !model.Limits.IsNull() && !model.Limits.IsUnknown() {
		if _, empty := model.Limits.Elements()[""]; empty {
			diags.AddError("Invalid node group limits", fmt.Sprintf("Node group %q: limits keys must not be empty.", name))
		}
	}

	for _, taint := range model.Taints {
		if known(taint.Effect) {
			if _, ok := taintEffectFromString(taint.Effect.ValueString()); !ok {
				diags.AddError("Invalid node group taint effect",
					fmt.Sprintf("Node group %q: taint effect must be one of %s, got %q.", name, strings.Join(validTaintEffects(), ", "), taint.Effect.ValueString()))
			}
		}
	}

	for _, accelerator := range model.Accelerators {
		if known(accelerator.Device) && accelerator.Device.ValueString() == "" {
			diags.AddError("Invalid node group accelerator", fmt.Sprintf("Node group %q: accelerator device must not be empty.", name))
		}
		if known(accelerator.Manufacturer) && accelerator.Manufacturer.ValueString() == "" {
			diags.AddError("Invalid node group accelerator", fmt.Sprintf("Node group %q: accelerator manufacturer must not be empty.", name))
		}
		if known(accelerator.Count) && accelerator.Count.ValueInt64() <= 0 {
			diags.AddError("Invalid node group accelerator",
				fmt.Sprintf("Node group %q: accelerator count must be greater than 0, got %d.", name, accelerator.Count.ValueInt64()))
		}
	}

	return diags
}

// known reports whether a value is set and fully known.
func known(value attr.Value) bool {
	return !value.IsNull() && !value.IsUnknown()
}
