---
page_title: "unionai_user_access Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about a Union.ai user access assignment.
---

# unionai_user_access (Data Source)

Retrieves information about a Union.ai user access assignment.

## Example Usage

```terraform
data "unionai_user_access" "example" {
  id = "user-access-id"
}

output "assigned_user" {
  value = data.unionai_user_access.example.user_id
}
```

## Schema

### Required

- `id` (String) The unique identifier of the user access assignment.

### Read-Only

- `user_id` (String) The ID of the user.
- `policy_id` (String) The ID of the policy assigned to the user.
