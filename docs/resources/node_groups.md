---
page_title: "unionai_node_groups Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages the complete set of node groups declared on a customer-managed cluster.
---

# unionai_node_groups (Resource)

Manages the complete set of node groups declared on a cluster. Union uses the declared node groups to know what capacity a cluster offers: instance types, node counts, labels, taints, accelerators and limits.

~> **This resource is for customer-managed clusters.** Union manages the node groups of a Union-provisioned (BYOC) cluster, and the control plane refuses writes to them with a permission error. The same goes for a cluster whose type was never recorded. To write a customer-managed cluster's node groups, the caller needs `MANAGE_CLUSTER` on the cluster.

~> **One resource owns every node group on a cluster.** Each write replaces the whole set, so a group missing from the configuration is deleted, including one created outside Terraform. Declare exactly one `unionai_node_groups` per cluster. On create, Terraform warns about any existing groups the configuration drops.

### Concurrent changes

The control plane keeps a change counter for each cluster's node groups, which this resource stores as `generation`. Every update sends it back. If the node groups changed outside Terraform since the last refresh, the write is refused and the apply fails with *Node groups changed outside Terraform*. Run `terraform plan` to see the current groups, then apply again. Terraform never retries the write on its own.

### Destroy

Destroying the resource writes an empty set of node groups, so the cluster declares none afterwards. The cluster itself is untouched.

## Example Usage

```terraform
resource "unionai_node_groups" "prod" {
  cluster = "prod-us-east-2"

  node_group {
    name                = "cpu-general"
    min_nodes           = 0
    max_nodes           = 10
    node_disk_size_gbs  = 100
    node_instance_types = ["m6i.2xlarge"]
    capacity_types      = ["ON_DEMAND"]
    labels              = { workload = "general" }
  }

  node_group {
    name                = "gpu-a100"
    min_nodes           = 0
    max_nodes           = 4
    node_disk_size_gbs  = 500
    node_instance_types = ["p4d.24xlarge"]
    capacity_types      = ["SPOT", "ON_DEMAND"]

    taint {
      key    = "nvidia.com/gpu"
      value  = "present"
      effect = "NoSchedule"
    }

    accelerator {
      device       = "A100"
      manufacturer = "NVIDIA"
      count        = 8
    }
  }
}
```

### Karpenter-style group with limits

```terraform
resource "unionai_node_groups" "staging" {
  cluster = "staging-us-west-2"

  node_group {
    name                = "general-arm"
    node_disk_size_gbs  = 100
    node_instance_types = ["m7g.2xlarge", "m7g.4xlarge"]
    capacity_types      = ["SPOT", "ON_DEMAND"]
    architecture        = "ARM64"
    weight              = 50
    max_pods            = 110
    limits              = { cpu = "1000", memory = "4000Gi" }
  }
}
```

## Schema

### Required

- `cluster` (String) Name of the cluster whose node groups this resource manages. Changing this forces replacement.

### Optional

- `org` (String) Organization that owns the cluster. Defaults to the provider's organization. Changing this forces replacement.
- `node_group` (Block Set) A node group. Groups are matched by `name`, so their order in the configuration does not matter. Leaving out every block declares no node groups. See [below for nested schema](#nestedblock--node_group).

### Read-Only

- `id` (String) Identifier in the form `org/cluster`.
- `generation` (Number) The cluster's node group change counter, as of the last read or write. Sent back on every update, so a write is refused if the node groups changed outside Terraform in the meantime.

<a id="nestedblock--node_group"></a>
### Nested Schema for `node_group`

Required:

- `name` (String) Node group name. Must match `^[a-zA-Z][a-zA-Z0-9-]{0,99}$` and be unique within the cluster.
- `node_disk_size_gbs` (Number) Disk size of each node, in GB. At least `1`.
- `node_instance_types` (List of String) Instance types the group may launch, e.g. `["m6i.2xlarge"]`. At least one.

Optional:

- `min_nodes` (Number) Minimum number of nodes. Unset means `0`.
- `max_nodes` (Number) Maximum number of nodes. Unset or `0` means the group has no node-count bound. A Karpenter NodePool, for example, is bounded by `limits` instead. When both are set, `min_nodes` may not exceed `max_nodes`.
- `capacity_types` (List of String) Capacity types the group may launch nodes as: `ON_DEMAND`, `SPOT` or `RESERVED`. List more than one for a group that can fall back from one type to another.
- `spot_instances` (Boolean) Whether the group runs on spot instances. Prefer `capacity_types`, which can also express a fallback.
- `architecture` (String) CPU architecture of the nodes: `AMD64` or `ARM64`. Unset means unknown, not `AMD64`.
- `weight` (Number) Preference weight between `1` and `100`, as in Karpenter's `NodePool.spec.weight`. Higher is preferred. Unset means no weight.
- `max_pods` (Number) Pod slots on one node (the kubelet's max-pods). Unset or `0` means unknown.
- `labels` (Map of String) Kubernetes labels on the group's nodes.
- `limits` (Map of String) Totals the group may not exceed across all of its nodes, as in Karpenter's `NodePool.spec.limits`. Each entry maps a Kubernetes resource name to a quantity, e.g. `{ cpu = "1000", "nvidia.com/gpu" = "8" }`.
- `allocatable_resources` (Map of String) Allocatable resources on one node, keyed by `cpu`, `gpu`, `memory`, `storage` or `ephemeral_storage`, e.g. `{ cpu = "8", memory = "32Gi" }`.
- `taint` (Block List) A Kubernetes taint on the group's nodes. See [below for nested schema](#nestedblock--node_group--taint).
- `accelerator` (Block List) An accelerator type present on each node of the group. See [below for nested schema](#nestedblock--node_group--accelerator).

<a id="nestedblock--node_group--taint"></a>
### Nested Schema for `node_group.taint`

Required:

- `key` (String) Taint key.
- `effect` (String) Taint effect: `NoSchedule`, `PreferNoSchedule` or `NoExecute`.

Optional:

- `value` (String) Taint value.

<a id="nestedblock--node_group--accelerator"></a>
### Nested Schema for `node_group.accelerator`

Required:

- `device` (String) The cloud provider's name for the device, e.g. `A100` or `nvidia-tesla-t4`.
- `manufacturer` (String) The device manufacturer, e.g. `NVIDIA` or `AMD`.
- `count` (Number) Number of these devices on one node. Greater than `0`.

Optional:

- `partition_size` (String) Multi-instance partition size, e.g. `1g.5gb`. Unset means the device is not partitioned.

## Import

Import a cluster's node groups with `org/cluster`, or with just the cluster name to use the provider's organization:

```shell
terraform import unionai_node_groups.prod acme/prod-us-east-2
terraform import unionai_node_groups.prod prod-us-east-2
```

Importing adopts whatever groups the cluster declares now. Copy them into the configuration before the next apply, or the apply deletes them.
