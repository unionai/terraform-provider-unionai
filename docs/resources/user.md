---
page_title: "unionai_user Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages a Union.ai user.
---

# unionai_user (Resource)

Manages a Union.ai user. Users are members of your organization who can access Union.ai resources.

**Note:** Changing the `first_name`, `last_name`, or `email` attributes will force replacement of the resource.

## Example Usage

```terraform
resource "unionai_user" "example" {
  first_name = "Jane"
  last_name  = "Doe"
  email      = "jane.doe@example.com"
}
```

## Schema

### Required

- `first_name` (String) The user's first name. Changing this forces a new resource to be created.
- `last_name` (String) The user's last name. Changing this forces a new resource to be created.
- `email` (String) The user's email address. Changing this forces a new resource to be created.

### Read-Only

- `id` (String) The unique identifier of the user.

## Import

Users can be imported using their ID:

```shell
terraform import unionai_user.example user-id
```
