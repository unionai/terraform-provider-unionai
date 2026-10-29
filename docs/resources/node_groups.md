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

### Node groups derived from Karpenter NodePools

On an EKS cluster where Karpenter provides the capacity, read the node groups off the `EC2NodeClass` and `NodePool` manifests instead of repeating their values, so the two cannot drift apart. Karpenter spells capacity types and architectures in lowercase (`on-demand`, `amd64`), so the example converts them. A NodePool is bounded by `limits` rather than a node count, so `min_nodes` and `max_nodes` stay unset. This needs the `kubernetes` provider configured for the cluster.

```terraform
locals {
  karpenter_node_pools = {
    general = {
      weight             = 10
      instance_types     = ["m7i.2xlarge", "m7i.4xlarge"]
      capacity_types     = ["spot", "on-demand"]
      limits             = { cpu = "1000", memory = "4000Gi" }
      labels             = { workload = "general" }
      taints             = []
      extra_requirements = []
    }
    gpu = {
      weight         = 50
      instance_types = ["g5.2xlarge", "g5.4xlarge"]
      capacity_types = ["on-demand"]
      limits         = { cpu = "256", memory = "1024Gi", "nvidia.com/gpu" = "16" }
      labels         = { workload = "gpu" }
      taints         = [{ key = "nvidia.com/gpu", value = "present", effect = "NoSchedule" }]
      extra_requirements = [
        { key = "karpenter.k8s.aws/instance-gpu-name", operator = "In", values = ["a10g"] },
        { key = "karpenter.k8s.aws/instance-gpu-manufacturer", operator = "In", values = ["nvidia"] },
        { key = "karpenter.k8s.aws/instance-gpu-count", operator = "In", values = ["1"] },
      ]
    }
  }
}

resource "kubernetes_manifest" "karpenter_node_class" {
  manifest = {
    apiVersion = "karpenter.k8s.aws/v1"
    kind       = "EC2NodeClass"
    metadata   = { name = "default" }
    spec = {
      role                       = "KarpenterNodeRole-prod-us-east-2"
      amiSelectorTerms           = [{ alias = "al2023@latest" }]
      subnetSelectorTerms        = [{ tags = { "karpenter.sh/discovery" = "prod-us-east-2" } }]
      securityGroupSelectorTerms = [{ tags = { "karpenter.sh/discovery" = "prod-us-east-2" } }]
      kubelet                    = { maxPods = 110 }
      blockDeviceMappings = [{
        deviceName = "/dev/xvda"
        ebs        = { volumeSize = "200Gi", volumeType = "gp3" }
      }]
    }
  }
}

resource "kubernetes_manifest" "karpenter_node_pool" {
  for_each = local.karpenter_node_pools

  manifest = {
    apiVersion = "karpenter.sh/v1"
    kind       = "NodePool"
    metadata   = { name = each.key }
    spec = {
      weight = each.value.weight
      limits = each.value.limits
      template = {
        metadata = { labels = each.value.labels }
        spec = {
          nodeClassRef = {
            group = "karpenter.k8s.aws"
            kind  = "EC2NodeClass"
            name  = kubernetes_manifest.karpenter_node_class.manifest.metadata.name
          }
          taints = each.value.taints
          requirements = concat([
            { key = "node.kubernetes.io/instance-type", operator = "In", values = each.value.instance_types },
            { key = "karpenter.sh/capacity-type", operator = "In", values = each.value.capacity_types },
            { key = "kubernetes.io/arch", operator = "In", values = ["amd64"] },
          ], each.value.extra_requirements)
        }
      }
    }
  }
}

locals {
  karpenter_node_class = kubernetes_manifest.karpenter_node_class.manifest.spec

  # Each NodePool's spec, with its requirements keyed by label for easy lookup.
  karpenter_node_groups = {
    for name, pool in kubernetes_manifest.karpenter_node_pool : name => {
      spec         = pool.manifest.spec
      requirements = { for requirement in pool.manifest.spec.template.spec.requirements : requirement.key => requirement.values }
    }
  }
}

resource "unionai_node_groups" "karpenter" {
  cluster = "prod-us-east-2"

  dynamic "node_group" {
    for_each = local.karpenter_node_groups

    content {
      name                = node_group.key
      node_instance_types = node_group.value.requirements["node.kubernetes.io/instance-type"]
      # Karpenter spells these "on-demand" and "amd64"; Union uses "ON_DEMAND" and "AMD64".
      capacity_types = [for capacity_type in node_group.value.requirements["karpenter.sh/capacity-type"] : upper(replace(capacity_type, "-", "_"))]
      architecture   = upper(one(node_group.value.requirements["kubernetes.io/arch"]))
      # A NodePool is bounded by limits, not a node count, so min_nodes and max_nodes stay unset.
      weight             = node_group.value.spec.weight
      limits             = node_group.value.spec.limits
      labels             = node_group.value.spec.template.metadata.labels
      max_pods           = local.karpenter_node_class.kubelet.maxPods
      node_disk_size_gbs = tonumber(trimsuffix(local.karpenter_node_class.blockDeviceMappings[0].ebs.volumeSize, "Gi"))

      dynamic "taint" {
        for_each = node_group.value.spec.template.spec.taints
        content {
          key    = taint.value.key
          value  = taint.value.value
          effect = taint.value.effect
        }
      }

      dynamic "accelerator" {
        for_each = contains(keys(node_group.value.requirements), "karpenter.k8s.aws/instance-gpu-name") ? [node_group.value.requirements] : []
        content {
          device       = upper(one(accelerator.value["karpenter.k8s.aws/instance-gpu-name"]))
          manufacturer = upper(one(accelerator.value["karpenter.k8s.aws/instance-gpu-manufacturer"]))
          count        = tonumber(one(accelerator.value["karpenter.k8s.aws/instance-gpu-count"]))
        }
      }
    }
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
