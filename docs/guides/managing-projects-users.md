---
page_title: "Managing Projects and Users"
subcategory: "Common Scenarios"
---

# Managing Projects and Users

This guide demonstrates how to manage projects and users in Union.ai using Terraform, a common scenario for teams setting up their infrastructure.

## Overview

In this guide, you'll learn how to:

- Create multiple projects (each with built-in development, staging, and production domains)
- Create user accounts
- Use built-in or custom roles
- Assign users to projects with appropriate roles and domains
- Manage user access programmatically

## Example: Setting Up a Multi-Project Environment

### 1. Define Your Projects

Create multiple projects for different teams or purposes. Each project automatically includes three built-in domains: `development`, `staging`, and `production`.

```terraform
resource "unionai_project" "ml_training" {
  name        = "ml-training"
  description = "Machine learning model training workflows"
}

resource "unionai_project" "data_processing" {
  name        = "data-processing"
  description = "Data pipeline and ETL workflows"
}
```

### 2. Create User Accounts

Define users who will access the projects:

```terraform
resource "unionai_user" "data_scientist" {
  email      = "data.scientist@example.com"
  name       = "Data Scientist"
  given_name = "Data"
  family_name = "Scientist"
}

resource "unionai_user" "ml_engineer" {
  email      = "ml.engineer@example.com"
  name       = "ML Engineer"
  given_name = "ML"
  family_name = "Engineer"
}

resource "unionai_user" "admin" {
  email      = "admin@example.com"
  name       = "Admin User"
  given_name = "Admin"
  family_name = "User"
}
```

### 3. Reference Built-in Roles

Union.ai provides three built-in roles. Load them using data sources:

```terraform
data "unionai_role" "admin" {
  name = "admin"
}

data "unionai_role" "contributor" {
  name = "contributor"
}

data "unionai_role" "viewer" {
  name = "viewer"
}
```

Alternatively, you can create custom roles with specific permissions:

```terraform
resource "unionai_role" "ml_developer" {
  name        = "ml-developer"
  description = "ML-specific development permissions"
  actions = [
    "view_flyte_executions",
    "create_flyte_executions",
    "view_flyte_inventory",
    "write_flyte_inventory",
  ]
}
```

### 4. Create Policies

Define access policies that bind roles to specific projects and domains:

```terraform
# Admin access to ML training across all domains
resource "unionai_policy" "ml_training_admin" {
  name = "ml-training-admin"

  project {
    id      = unionai_project.ml_training.id
    role_id = data.unionai_role.admin.id
    domains = ["development", "staging", "production"]
  }
}

# Contributor access to data processing dev and staging
resource "unionai_policy" "data_processing_contributor" {
  name = "data-processing-contributor"

  project {
    id      = unionai_project.data_processing.id
    role_id = data.unionai_role.contributor.id
    domains = ["development", "staging"]
  }
}

# Viewer access to data processing production
resource "unionai_policy" "data_processing_viewer" {
  name = "data-processing-viewer"

  project {
    id      = unionai_project.data_processing.id
    role_id = data.unionai_role.viewer.id
    domains = ["production"]
  }
}
```

### 5. Assign Users to Projects

Grant users access to projects using the policies:

```terraform
# Admin has full access to ML training
resource "unionai_user_access" "admin_ml_training" {
  user   = unionai_user.admin.id
  policy = unionai_policy.ml_training_admin.id
}

# Data scientist has admin access to ML training
resource "unionai_user_access" "ds_ml_training" {
  user   = unionai_user.data_scientist.id
  policy = unionai_policy.ml_training_admin.id
}

# ML engineer has contributor access to data processing (dev/staging)
resource "unionai_user_access" "mle_data_processing" {
  user   = unionai_user.ml_engineer.id
  policy = unionai_policy.data_processing_contributor.id
}

# Both data scientist and ML engineer can view data processing production
resource "unionai_user_access" "ds_data_processing_viewer" {
  user   = unionai_user.data_scientist.id
  policy = unionai_policy.data_processing_viewer.id
}

resource "unionai_user_access" "mle_data_processing_viewer" {
  user   = unionai_user.ml_engineer.id
  policy = unionai_policy.data_processing_viewer.id
}
```

## Using Data Sources

You can also reference existing resources using data sources:

```terraform
# Reference an existing project
data "unionai_project" "existing_project" {
  name = "legacy-project"
}

# Reference an existing user
data "unionai_user" "existing_user" {
  email = "existing.user@example.com"
}

# Grant access to existing resources
resource "unionai_user_access" "existing_access" {
  user   = data.unionai_user.existing_user.id
  policy = unionai_policy.data_processing_viewer.id
}
```

## Best Practices

1. **Use Built-in Roles First**: Start with the built-in roles (admin, contributor, viewer) before creating custom roles
2. **Leverage Built-in Domains**: Use the three built-in domains (development, staging, production) within each project rather than creating separate projects for each environment
3. **Use Variables**: Define user emails and project names as variables for easier management
4. **Domain-Based Access Control**: Use stricter policies in production domains than in development or staging
5. **Version Control**: Keep your Terraform configuration in Git
6. **State Management**: Use remote state (S3, Terraform Cloud) for team collaboration
7. **Security**: Never commit API keys or sensitive data to version control

## Complete Example

See the [examples/resources](../../examples/resources/) directory for a complete working example.
