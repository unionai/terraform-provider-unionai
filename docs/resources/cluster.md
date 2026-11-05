---
page_title: "unionai_cluster Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Registers a dataplane cluster with the Union.ai control plane.
---

# unionai_cluster (Resource)

Registers a dataplane cluster with the control plane. This is the record the dataplane's operator heartbeats against and reports its status to. Until it exists, the operator's `Heartbeat` and `UpdateStatus` calls fail with `NotFound: missing entity of type Cluster`. The resource does the same thing as `flyte create cluster <name>`, without an interactive login.

`name` must match the cluster name the dataplane is configured with.

~> **A cluster owns its name in the queue namespace.** Creating a cluster also creates a queue of the same name, in the same cluster pool, that routes to this cluster only. Creation fails if a queue of that name already exists. The name `default` is reserved and cannot be used.

### Existing clusters

Terraform does not adopt a cluster that already exists. If the name is taken, for example by a cluster registered with `flyte create cluster`, the apply fails with *Cluster already exists*. Import the cluster instead (see below).

### Changing the cluster pool

Changing `cluster_pool_name` calls `UpdateCluster`, which moves the cluster and its co-named queue together. The control plane only accepts the move when:

- the co-named queue is drained,
- the target pool exists and is live, and
- no other live queue pins the cluster.

If any of these fails, the apply fails and the cluster stays where it was. Changing `name` or `org` replaces the cluster.

### Destroy

~> **Destroying a cluster soft-deletes it.** Destroy calls `DeleteCluster`. A `drained` cluster goes straight to `deleted`. An `active` or `draining` cluster goes to `deleting`, and the control plane finishes the deletion once it has cleaned up the cluster's workloads. Its co-named queue is deleted with it. Afterwards the dataplane's heartbeats are refused.

| Situation | Outcome |
| --- | --- |
| Cluster is `active`, `draining` or `drained` | Terraform calls `DeleteCluster`, removes state, and warns that the cluster was soft-deleted |
| Cluster is already `deleting` or `deleted`, or missing | Terraform removes state |
| Another live queue pins the cluster | Destroy fails and the resource stays in state; unpin the cluster first |

A soft-deleted cluster keeps its name reserved. Applying the same configuration again fails with *Cluster already exists* until the cluster is undeleted outside Terraform and imported.

Apps assigned to the cluster are not evicted or reassigned on deletion. The dataplane resources themselves are not touched.

## Example Usage

```terraform
resource "unionai_cluster" "example" {
  name = "prod-us-east-2"
}

resource "unionai_cluster" "gpu" {
  name              = "gpu-us-west-2"
  cluster_pool_name = "gpu-pool"
}
```

## Schema

### Required

- `name` (String) Cluster name. Must match the cluster name the dataplane is configured with. Unique within the organization, and shared with the queue namespace: it cannot be the name of an existing queue, or `default`. Changing this forces replacement.

### Optional

- `org` (String) Organization that owns the cluster. Defaults to the provider's organization. Changing this forces replacement.
- `cluster_pool_name` (String) Cluster pool the cluster belongs to. Defaults to `default`; any other pool must already exist. Changing it moves the cluster and its co-named queue, which must be drained first.

### Read-Only

- `id` (String) Identifier in the form `org/name`.
- `state` (String) Cluster lifecycle state: `active`, `draining`, `drained`, `deleting`, or `deleted`. Managed by the control plane.

## Import

Clusters can be imported by name, in the provider's organization, or as `org/name`:

```shell
terraform import unionai_cluster.example prod-us-east-2
terraform import unionai_cluster.example acme/prod-us-east-2
```

This is also the way to adopt a cluster registered with `flyte create cluster`. Soft-deleted clusters cannot be imported until they are undeleted.
