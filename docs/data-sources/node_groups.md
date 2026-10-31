---
page_title: "unionai_node_groups Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Reads the node groups declared on a cluster.
---

# unionai_node_groups (Data Source)

Reads the node groups declared on a cluster, exactly as stored. Unlike the console, it filters out no labels or taints. It works for any cluster the caller can read, including Union-provisioned (BYOC) clusters whose node groups the `unionai_node_groups` resource cannot manage.

## Example Usage

```terraform
data "unionai_node_groups" "prod" {
  cluster = "prod-us-east-2"
}

output "prod_gpu_groups" {
  value = [for group in data.unionai_node_groups.prod.node_group : group.name if length(group.accelerator) > 0]
}
```

## Schema

### Required

- `cluster` (String) Cluster name.

### Optional

- `org` (String) Organization that owns the cluster. Defaults to the provider's organization.

### Read-Only

- `id` (String) Identifier in the form `org/cluster`.
- `generation` (Number) The cluster's node group change counter. `0` means the node groups have never been written.
- `node_group` (List of Object) The cluster's node groups, sorted by name. Each has the attributes of the resource's `node_group` block (`name`, `min_nodes`, `max_nodes`, `node_disk_size_gbs`, `node_instance_types`, `spot_instances`, `capacity_types`, `architecture`, `weight`, `max_pods`, `labels`, `limits`, `allocatable_resources`, `taint` and `accelerator`). Unset numbers read as `0`, and unset collections as empty. `architecture` and `weight` are null when unset.
