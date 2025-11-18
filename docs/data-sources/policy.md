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

- `description` (String) The description of the policy.
- `roles` (List of Object) The roles assigned by this policy (see [below for nested schema](#nestedatt--roles))

<a id="nestedatt--roles"></a>
### Nested Schema for `roles`

Read-Only:

- `role_id` (String) The ID of the role.
- `resource` (Object) The resource to which the role applies (see [below for nested schema](#nestedobjatt--roles--resource))

<a id="nestedobjatt--roles--resource"></a>
### Nested Schema for `roles.resource`

Read-Only:

- `org_id` (String) The organization identifier (if this is an organization-level assignment).
- `domain_id` (String) The domain identifier (if this is a domain-level assignment).
- `project_id` (String) The project identifier (if this is a project-level assignment).
