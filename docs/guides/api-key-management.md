---
page_title: "API Key and Application Management"
subcategory: "Common Scenarios"
---

# API Key and Application Management

This guide shows you how to manage API keys and OAuth applications for programmatic access to Union.ai using Terraform.

## Overview

Union.ai supports two types of programmatic access:

1. **API Keys**: For service accounts and automation
2. **OAuth Applications**: For third-party integrations

## Managing API Keys

API keys provide a simple way to authenticate automated systems and CI/CD pipelines.

### Creating API Keys for Different Purposes

```terraform
# API key for CI/CD pipeline
resource "unionai_api_key" "ci_cd" {
  name        = "github-actions-ci"
  description = "API key for GitHub Actions CI/CD pipeline"
}

# API key for monitoring
resource "unionai_api_key" "monitoring" {
  name        = "monitoring-service"
  description = "API key for monitoring and alerting system"
}

# API key for data ingestion
resource "unionai_api_key" "data_ingestion" {
  name        = "data-ingestion-service"
  description = "API key for automated data ingestion workflows"
}
```

### Storing API Keys Securely

After creation, store the API key values in a secure location:

```terraform
# Output API keys to be stored in secrets manager
output "ci_cd_api_key" {
  description = "API key for CI/CD - store this in your secrets manager"
  value       = unionai_api_key.ci_cd.key
  sensitive   = true
}

output "monitoring_api_key" {
  description = "API key for monitoring - store this in your secrets manager"
  value       = unionai_api_key.monitoring.key
  sensitive   = true
}
```

To view the sensitive output:

```bash
terraform output -raw ci_cd_api_key
```

### Integrating with Secrets Managers

#### AWS Secrets Manager

```terraform
resource "aws_secretsmanager_secret" "unionai_ci_key" {
  name        = "unionai/ci-cd-api-key"
  description = "Union.ai API key for CI/CD"
}

resource "aws_secretsmanager_secret_version" "unionai_ci_key" {
  secret_id     = aws_secretsmanager_secret.unionai_ci_key.id
  secret_string = unionai_api_key.ci_cd.key
}
```

#### HashiCorp Vault

```terraform
resource "vault_generic_secret" "unionai_ci_key" {
  path = "secret/unionai/ci-cd"

  data_json = jsonencode({
    api_key = unionai_api_key.ci_cd.key
  })
}
```

#### GitHub Secrets (via GitHub provider)

```terraform
resource "github_actions_secret" "unionai_api_key" {
  repository      = "my-repo"
  secret_name     = "UNIONAI_API_KEY"
  plaintext_value = unionai_api_key.ci_cd.key
}
```

## Managing OAuth Applications

OAuth applications enable third-party integrations with proper authorization flows.

### Creating an OAuth Application

```terraform
resource "unionai_application" "web_dashboard" {
  name        = "custom-web-dashboard"
  description = "Custom web dashboard for workflow monitoring"

  # OAuth configuration would go here based on your application settings
}

# Grant the application access to specific projects
resource "unionai_application_access" "dashboard_dev" {
  application_id = unionai_application.web_dashboard.id
  policy_id      = unionai_policy.dev_viewer.id
}

resource "unionai_application_access" "dashboard_prod" {
  application_id = unionai_application.web_dashboard.id
  policy_id      = unionai_policy.prod_viewer.id
}
```

## Reading Existing API Keys

Use data sources to reference existing API keys:

```terraform
data "unionai_api_key" "existing_key" {
  name = "legacy-api-key"
}

# Use the existing key's information
output "existing_key_id" {
  value = data.unionai_api_key.existing_key.id
}
```

## API Key Rotation Strategy

Implement a rotation strategy using Terraform:

```terraform
variable "api_key_generation" {
  description = "Increment this to rotate API keys"
  type        = number
  default     = 1
}

resource "unionai_api_key" "rotating_key" {
  name        = "service-key-gen-${var.api_key_generation}"
  description = "API key generation ${var.api_key_generation}"
}

# When rotating:
# 1. Increment api_key_generation
# 2. Apply to create new key
# 3. Update services to use new key
# 4. Remove old key from state
```

## Best Practices

### 1. Naming Convention

Use descriptive names that indicate purpose:

```terraform
resource "unionai_api_key" "prod_deployment" {
  name        = "prod-deployment-${var.environment}"
  description = "API key for production deployments in ${var.environment}"
}
```

### 2. Lifecycle Management

Set lifecycle rules to prevent accidental deletion:

```terraform
resource "unionai_api_key" "critical_service" {
  name        = "critical-production-service"
  description = "API key for critical production service"

  lifecycle {
    prevent_destroy = true
  }
}
```

### 3. Tagging and Documentation

Document the purpose of each API key:

```terraform
locals {
  api_keys = {
    ci_cd = {
      name        = "github-actions-ci"
      description = "GitHub Actions CI/CD pipeline"
      owner       = "devops-team@example.com"
      created_for = "Automated testing and deployment"
    }
    monitoring = {
      name        = "datadog-monitoring"
      description = "Datadog monitoring integration"
      owner       = "sre-team@example.com"
      created_for = "Metrics collection and alerting"
    }
  }
}

resource "unionai_api_key" "managed_keys" {
  for_each    = local.api_keys
  name        = each.value.name
  description = "${each.value.description} (Owner: ${each.value.owner})"
}
```

### 4. Separate Environments

Use different API keys for different environments:

```terraform
resource "unionai_api_key" "ci" {
  for_each = toset(["dev", "staging", "prod"])

  name        = "ci-cd-${each.key}"
  description = "CI/CD API key for ${each.key} environment"
}
```

## Security Checklist

- [ ] Never commit API keys to version control
- [ ] Store API keys in a secrets manager
- [ ] Use different API keys for different environments
- [ ] Implement key rotation policies
- [ ] Monitor API key usage
- [ ] Use least-privilege access with policies
- [ ] Document the purpose of each API key
- [ ] Set up alerts for unusual API key activity

## Example: Complete CI/CD Setup

```terraform
# Create API key for CI/CD
resource "unionai_api_key" "github_actions" {
  name        = "github-actions-main"
  description = "API key for GitHub Actions workflows"
}

# Store in GitHub secrets
resource "github_actions_secret" "unionai_key" {
  repository      = "my-workflows-repo"
  secret_name     = "UNIONAI_API_KEY"
  plaintext_value = unionai_api_key.github_actions.key
}

# Also backup in AWS Secrets Manager
resource "aws_secretsmanager_secret" "github_unionai_key" {
  name = "github-actions/unionai-api-key"
}

resource "aws_secretsmanager_secret_version" "github_unionai_key" {
  secret_id     = aws_secretsmanager_secret.github_unionai_key.id
  secret_string = jsonencode({
    api_key     = unionai_api_key.github_actions.key
    description = unionai_api_key.github_actions.description
    created_at  = timestamp()
  })
}
```

## Troubleshooting

### API Key Not Working

1. Verify the key exists in Union.ai
2. Check that the key has the necessary permissions via policies
3. Ensure the `allowed_orgs` provider setting matches your key's organization

### Permission Denied

Check that you've assigned appropriate policies to your API key or application:

```terraform
resource "unionai_application_access" "check_permissions" {
  application_id = unionai_application.my_app.id
  policy_id      = unionai_policy.required_access.id
}
```

## Related Resources

- [unionai_api_key](/docs/resources/unionai_api_key.md) - API Key resource documentation
- [unionai_application](/docs/resources/unionai_application.md) - Application resource documentation
- [unionai_application_access](/docs/resources/unionai_application_access.md) - Application access documentation
