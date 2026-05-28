---
page_title: "unionai_project_domain_attributes Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages cluster resource attributes for a project-domain pair.
---

# unionai_project_domain_attributes (Resource)

Manages the cluster resource attributes (matchable attributes of type `CLUSTER_RESOURCE`) for a project-domain pair.

The attribute map is substituted into the cluster resource templates that Flyte renders for the project-domain namespace. The most common use is setting `defaultIamRole` to bind a per-project IAM role to the namespace's default ServiceAccount, giving each project-domain its own scoped cloud identity.

## Example Usage

```terraform
resource "unionai_project" "test" {
  name        = "test"
  description = "Test Project"
}

# Bind a per-project IAM role to the project-domain namespace's default
# ServiceAccount by setting the defaultIamRole cluster resource template variable.
resource "unionai_project_domain_attributes" "test" {
  project = unionai_project.test.id
  domain  = "development"

  attributes = {
    defaultIamRole = "arn:aws:iam::123456789012:role/my-project-development-role"
  }
}
```

## Schema

### Required

- `project` (String) Project identifier the attributes apply to.
- `domain` (String) Domain the attributes apply to (e.g. `development`, `staging`, `production`).
- `attributes` (Map of String) Cluster resource template variables to substitute, as case-sensitive key/value pairs (e.g. `{ defaultIamRole = "arn:aws:iam::123456789012:role/my-role" }`).

### Read-Only

- `id` (String) Resource identifier, in the form `{project}/{domain}`.

## Import

Project domain attributes can be imported using `{project}/{domain}`:

```shell
terraform import unionai_project_domain_attributes.test my-project/development
```
