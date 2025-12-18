---
page_title: "unionai_secret Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about a Union.ai secret.
---

# unionai_secret (Data Source)

Retrieves information about a Union.ai secret. This data source returns metadata about the secret but does not expose the actual secret value for security reasons.

## Example Usage

### Organization-level Secret

```terraform
data "unionai_secret" "api_key" {
  name = "github-api-token"
}

output "secret_type" {
  value = data.unionai_secret.api_key.type
}

output "created_time" {
  value = data.unionai_secret.api_key.created_time
}
```

### Domain-scoped Secret

```terraform
data "unionai_secret" "dev_db_password" {
  name   = "database-password"
  domain = "development"
}
```

### Project-scoped Secret

```terraform
data "unionai_secret" "docker_creds" {
  name    = "docker-registry-creds"
  project = "my-project"
}
```

## Schema

### Required

- `name` (String) The name of the secret to retrieve.

### Optional

- `organization` (String) Organization scope for the secret. Defaults to the provider's organization.
- `domain` (String) Domain scope for the secret.
- `project` (String) Project scope for the secret.

### Read-Only

- `id` (String) The unique identifier of the secret.
- `type` (String) The type of the secret (`generic` or `image_pull_secret`).
- `created_time` (String) Timestamp when the secret was created.

**Note:** The actual secret value is not returned by this data source for security reasons.
