---
page_title: "unionai_secrets Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves a list of Union.ai secrets.
---

# unionai_secrets (Data Source)

Retrieves a list of Union.ai secrets with their identifiers. This data source returns all secrets accessible at the specified scope level. The results are sorted by organization, domain, project, and name for consistent ordering.

Since secrets are uniquely identified by the combination of `name`, `organization`, `domain`, and `project`, multiple secrets with the same name can exist at different scope levels.

## Example Usage

### List All Organization Secrets

```terraform
data "unionai_secrets" "all" {}

output "all_secrets" {
  value = data.unionai_secrets.all.secrets
}
```

### List Secrets in a Specific Domain

```terraform
data "unionai_secrets" "dev_secrets" {
  domain = "development"
}

output "dev_secret_count" {
  value = length(data.unionai_secrets.dev_secrets.secrets)
}
```

### List Secrets in a Specific Project

```terraform
data "unionai_secrets" "project_secrets" {
  project = "my-project"
}

# Iterate over secrets to create outputs
output "project_secret_names" {
  value = [for s in data.unionai_secrets.project_secrets.secrets : s.name]
}
```

### Filter and Process Secrets

```terraform
data "unionai_secrets" "all" {}

# Find all image pull secrets
locals {
  image_pull_secrets = [
    for s in data.unionai_secrets.all.secrets : s
    if can(regex(".*-registry.*", s.name))
  ]
}

output "registry_secrets" {
  value = local.image_pull_secrets
}
```

## Schema

### Optional

- `organization` (String) Organization scope for listing secrets. Defaults to the provider's organization.
- `domain` (String) Domain scope for filtering secrets. If specified, returns secrets at this domain level.
- `project` (String) Project scope for filtering secrets. If specified, returns secrets at this project level.

### Read-Only

- `secrets` (List of Object) List of secrets with their identifiers. Each object contains:
  - `name` (String) The name of the secret.
  - `organization` (String) The organization scope.
  - `domain` (String) The domain scope (empty string if organization-level).
  - `project` (String) The project scope (empty string if not project-scoped).

## Notes

- The list is sorted by organization, domain, project, and name for deterministic output.
- Empty strings for `domain` and `project` indicate organization-level or domain-level secrets respectively.
- The actual secret values are not returned for security reasons.
- The data source handles pagination automatically to retrieve all secrets.
