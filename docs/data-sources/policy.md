---
page_title: "unionai_policy Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about a Union.ai policy.
---

# unionai_policy (Data Source)

Retrieves information about a Union.ai policy.

## Example Usage

```terraform
data "unionai_policy" "example" {
  id = "policy-id"
}

output "policy_name" {
  value = data.unionai_policy.example.name
}
```

## Schema

### Required

- `id` (String) The unique identifier of the policy.

### Read-Only

- `name` (String) The name of the policy.
- `description` (String) The description of the policy.
- `organization` (List of Object) Organization-level policy assignments.
- `project` (List of Object) Project-level policy assignments.
- `domain` (List of Object) Domain-level policy assignments.
