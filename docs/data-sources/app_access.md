---
page_title: "unionai_app_access Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about a Union.ai application access assignment.
---

# unionai_app_access (Data Source)

Retrieves information about a Union.ai application access assignment.

## Example Usage

```terraform
data "unionai_app_access" "example" {
  id = "app-access-id"
}

output "assigned_app" {
  value = data.unionai_app_access.example.app_id
}
```

## Schema

### Required

- `id` (String) The unique identifier of the application access assignment.

### Read-Only

- `app_id` (String) The ID of the application.
- `policy_id` (String) The ID of the policy assigned to the application.
