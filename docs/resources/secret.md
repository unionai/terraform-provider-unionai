---
page_title: "unionai_secret Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages a Union.ai secret.
---

# unionai_secret (Resource)

Manages a Union.ai secret. Secrets are used to securely store sensitive information like API keys, passwords, and credentials that can be accessed by workflows.

Secrets can be scoped at different levels:
- **Organization-level**: Available across the entire organization
- **Domain-level**: Available within a specific domain
- **Project-level**: Available only within a specific project

The uniqueness of a secret is determined by the combination of `name`, `organization`, `domain`, and `project`. Multiple secrets can have the same `name` as long as they exist at different scope levels.

**Note:** Changing the `name`, `organization`, `domain`, or `project` will force replacement of the resource.

## Example Usage

### String Secret (Organization-level)

```terraform
resource "unionai_secret" "api_key" {
  name  = "github-api-token"
  value = "ghp_xxxxxxxxxxxxxxxxxxxx"
  type  = "generic"
}
```

### Binary Secret (Base64-encoded)

```terraform
resource "unionai_secret" "service_account" {
  name         = "gcp-service-account"
  binary_value = base64encode(file("service-account.json"))
  type         = "generic"
}
```

### Image Pull Secret (Project-level)

```terraform
resource "unionai_secret" "docker_registry" {
  name    = "docker-registry-creds"
  project = "my-project"
  value   = jsonencode({
    auths = {
      "registry.example.com" = {
        username = "myuser"
        password = "mypassword"
      }
    }
  })
  type = "image_pull_secret"
}
```

### Domain-scoped Secret

```terraform
resource "unionai_secret" "dev_db_password" {
  name   = "database-password"
  domain = "development"
  value  = var.dev_db_password
}
```

## Schema

### Required

- `name` (String) The name of the secret. Must match pattern `^[-a-zA-Z0-9_]+$`. Changing this forces a new resource to be created.

### Optional

- `organization` (String) Organization scope for the secret. Defaults to the provider's organization. Changing this forces a new resource to be created.
- `domain` (String) Domain scope for the secret. Leave empty for organization-level secrets. Changing this forces a new resource to be created.
- `project` (String) Project scope for the secret. Leave empty for organization or domain-level secrets. Changing this forces a new resource to be created.
- `value` (String, Sensitive) String value of the secret. Mutually exclusive with `binary_value`. One of `value` or `binary_value` must be specified.
- `binary_value` (String, Sensitive) Base64-encoded binary value of the secret. Mutually exclusive with `value`. One of `value` or `binary_value` must be specified.
- `type` (String) Secret type. Valid values: `generic`, `image_pull_secret`. Defaults to `generic`.

### Read-Only

- `id` (String) The unique identifier of the secret (same as `name`).

## Import

Secrets can be imported using their name:

```shell
terraform import unionai_secret.example secret-name
```

For scoped secrets, you may need to set the scope attributes in your configuration before importing.
