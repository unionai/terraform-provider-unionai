---
page_title: "unionai_user_access Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages access policies for a Union.ai user.
---

# unionai_user_access (Resource)

Manages access policies for a Union.ai user. This resource assigns policies to users, granting them specific permissions.

## Example Usage

```terraform
resource "unionai_user" "data_scientist" {
  first_name = "Jane"
  last_name  = "Doe"
  email      = "jane.doe@example.com"
}

resource "unionai_policy" "dev_access" {
  name        = "developer-access"
  description = "Access for developers"

  project {
    id      = unionai_project.example.id
    role_id = unionai_role.developer.id
    domains = ["development"]
  }
}

resource "unionai_user_access" "jane_dev" {
  user_id   = unionai_user.data_scientist.id
  policy_id = unionai_policy.dev_access.id
}
```

## Schema

### Required

- `user_id` (String) The ID of the user to grant access to.
- `policy_id` (String) The ID of the policy to assign to the user.

### Read-Only

- `id` (String) The unique identifier of the user access assignment.

## Import

User access assignments can be imported using their ID:

```shell
terraform import unionai_user_access.example user-access-id
```
