---
page_title: "unionai_role Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about a Union.ai role.
---

# unionai_role (Data Source)

Retrieves information about a Union.ai role.

## Example Usage

```terraform
data "unionai_role" "admin" {
  id = "admin-role-id"
}

output "role_actions" {
  value = data.unionai_role.admin.actions
}
```

## Schema

### Required

- `id` (String) The unique identifier of the role.

### Read-Only

- `name` (String) The name of the role.
- `description` (String) The description of the role.
- `actions` (Set of String) The set of actions that this role grants.
