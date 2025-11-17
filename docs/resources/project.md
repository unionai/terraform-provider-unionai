---
page_title: "unionai_project Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages a Union.ai project.
---

# unionai_project (Resource)

Manages a Union.ai project. Projects are the top-level organizational unit in Union.ai.

## Example Usage

```terraform
resource "unionai_project" "example" {
  name        = "my-project"
  description = "My example project for data workflows"
}
```

## Schema

### Required

- `name` (String) The name of the project. This must be unique within your organization.

### Optional

- `description` (String) A description of the project.

### Read-Only

- `id` (String) The unique identifier of the project.

## Import

Projects can be imported using their ID:

```shell
terraform import unionai_project.example project-id
```
