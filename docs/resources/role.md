---
page_title: "unionai_role Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages a Union.ai role.
---

# unionai_role (Resource)

Manages a Union.ai role. Roles define a set of actions that can be performed within the organization.

**Note:** Changing any attribute will force replacement of the resource.

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
- `actions` (Set of String) The set of actions that this role grants. Changing this forces a new resource to be created. Common values: `administer_account`, `administer_project`, `create_flyte_executions`, `edit_cluster_related_attributes`, `edit_execution_related_attributes`, `edit_unused_attributes`, `manage_cluster`, `manage_permissions`, `register_flyte_inventory`, `view_flyte_executions`, `view_flyte_inventory`.

### Optional

- `description` (String) A description of the role. Changing this forces a new resource to be created.

### Read-Only

- `id` (String) The unique identifier of the role.

## Import

Roles can be imported using their ID:

```shell
terraform import unionai_role.example role-id
```
