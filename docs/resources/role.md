---
page_title: "unionai_role Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages a Union.ai role.
---

# unionai_role (Resource)

Manages a Union.ai role. Roles define a set of actions that can be performed within the organization.

**Note:** Changing `name` or `description` forces replacement of the resource. Changing `actions` adds and removes actions on the existing role.

## Example Usage

```terraform
resource "unionai_role" "example" {
  name        = "data-scientist"
  description = "Role for data scientists with workflow execution permissions"
  actions = [
    "create_flyte_executions",
    "view_flyte_inventory"
  ]
}
```

## Schema

### Required

- `name` (String) The name of the role. Changing this forces a new resource to be created.
- `actions` (Set of String) The set of actions that this role grants. Changes are applied in place, adding new actions before removing old ones. Common values: `administer_account`, `administer_project`, `create_flyte_executions`, `edit_cluster_related_attributes`, `edit_execution_related_attributes`, `edit_settings`, `edit_unused_attributes`, `manage_apps`, `manage_artifacts`, `manage_cluster`, `manage_permissions`, `manage_secrets`, `manage_triggers`, `manage_workspaces`, `register_flyte_inventory`, `view_apps`, `view_artifacts`, `view_clusters`, `view_flyte_executions`, `view_flyte_inventory`, `view_identities`, `view_permissions`, `view_secrets`, `view_settings`, `view_triggers`, `view_usage`, `view_workspaces`.

### Optional

- `description` (String) A description of the role. Changing this forces a new resource to be created.

### Read-Only

- `id` (String) The unique identifier of the role.

## Import

Roles can be imported using their ID:

```shell
terraform import unionai_role.example role-id
```
