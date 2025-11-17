---
page_title: "unionai_user Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about a Union.ai user.
---

# unionai_user (Data Source)

Retrieves information about a Union.ai user.

## Example Usage

```terraform
data "unionai_user" "example" {
  id = "user-id"
}

output "user_email" {
  value = data.unionai_user.example.email
}
```

## Schema

### Required

- `id` (String) The unique identifier of the user.

### Read-Only

- `first_name` (String) The user's first name.
- `last_name` (String) The user's last name.
- `email` (String) The user's email address.
