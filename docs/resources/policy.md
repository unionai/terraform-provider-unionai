---
page_title: "unionai_policy Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages a Union.ai policy.
---

# unionai_policy (Resource)

Manages a Union.ai policy. Policies assign roles to subjects (users or apps) at different resource scopes (organization, project, or domain).

## Example Usage

```terraform
# Organization-level policy
resource "unionai_policy" "org_admin" {
  name        = "org-admin-policy"
  description = "Grant admin role at organization level"

  organization {
    id      = "my-org"
    role_id = unionai_role.admin.id
  }
}

# Project-level policy with domain restriction
resource "unionai_policy" "project_dev" {
  name        = "project-dev-policy"
  description = "Grant developer role for specific project and domains"

  project {
    id      = unionai_project.example.id
    role_id = unionai_role.developer.id
    domains = ["development", "staging"]
  }
}

# Domain-level policy
resource "unionai_policy" "domain_viewer" {
  name        = "domain-viewer-policy"
  description = "Grant viewer role at domain level"

  domain {
    id      = "production"
    role_id = unionai_role.viewer.id
  }
}
```

## Schema

### Required

- `name` (String) The name of the policy.

### Optional

- `description` (String) A description of the policy.
- `organization` (Block List) Organization-level policy assignments (see [below for nested schema](#nestedblock--organization))
- `project` (Block List) Project-level policy assignments (see [below for nested schema](#nestedblock--project))
- `domain` (Block List) Domain-level policy assignments (see [below for nested schema](#nestedblock--domain))

### Read-Only

- `id` (String) The unique identifier of the policy.

<a id="nestedblock--organization"></a>
### Nested Schema for `organization`

Required:

- `id` (String) The organization identifier.
- `role_id` (String) The ID of the role to assign.

<a id="nestedblock--project"></a>
### Nested Schema for `project`

Required:

- `id` (String) The project identifier.
- `role_id` (String) The ID of the role to assign.

Optional:

- `domains` (Set of String) The domains within the project to which this policy applies. Valid values are: `production`, `staging`, `development`.

<a id="nestedblock--domain"></a>
### Nested Schema for `domain`

Required:

- `id` (String) The domain identifier.
- `role_id` (String) The ID of the role to assign.

## Import

Policies can be imported using their ID:

```shell
terraform import unionai_policy.example policy-id
```
