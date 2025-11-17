---
page_title: "unionai_project Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about a Union.ai project.
---

# unionai_project (Data Source)

Retrieves information about a Union.ai project.

## Example Usage

```terraform
data "unionai_project" "example" {
  id = "my-project-id"
}

output "project_name" {
  value = data.unionai_project.example.name
}
```

## Schema

### Required

- `id` (String) The unique identifier of the project.

### Read-Only

- `name` (String) The name of the project.
- `description` (String) The description of the project.
- `domain_ids` (Set of String) The set of domain identifiers associated with this project.
- `state` (String) The current state of the project.
