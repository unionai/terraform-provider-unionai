---
page_title: "Managing Projects and Users"
subcategory: "Common Scenarios"
---

# Managing Projects and Users

This guide demonstrates how to manage projects and users in Union.ai using Terraform, a common scenario for teams setting up their infrastructure.

## Overview

In this guide, you'll learn how to:

- Create multiple projects
- Create user accounts
- Assign users to projects with appropriate roles
- Manage user access programmatically

## Example: Setting Up a Multi-Project Environment

### 1. Define Your Projects

Create multiple projects for different teams or purposes:

```terraform
resource "unionai_project" "ml_training" {
  name        = "ml-training"
  description = "Machine learning model training workflows"
}

resource "unionai_project" "data_processing" {
  name        = "data-processing"
  description = "Data pipeline and ETL workflows"
}

resource "unionai_project" "production" {
  name        = "production"
  description = "Production workflows"
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

### 3. Define Roles

Create custom roles with specific permissions:

```terraform
resource "unionai_role" "project_admin" {
  name        = "project-admin"
  description = "Full access to project resources"
}

resource "unionai_role" "developer" {
  name        = "developer"
  description = "Read and execute workflows"
}

resource "unionai_role" "viewer" {
  name        = "viewer"
  description = "Read-only access"
}
```

### 4. Create Policies

Define access policies that combine roles with permissions:

```terraform
resource "unionai_policy" "ml_training_admin" {
  project_id = unionai_project.ml_training.id
  role_id    = unionai_role.project_admin.id
}

resource "unionai_policy" "data_processing_developer" {
  project_id = unionai_project.data_processing.id
  role_id    = unionai_role.developer.id
}

resource "unionai_policy" "production_viewer" {
  project_id = unionai_project.production.id
  role_id    = unionai_role.viewer.id
}
```

### 5. Assign Users to Projects

Grant users access to projects using the policies:

```terraform
# Admin has full access to all projects
resource "unionai_user_access" "admin_ml_training" {
  user_id   = unionai_user.admin.id
  policy_id = unionai_policy.ml_training_admin.id
}

resource "unionai_user_access" "admin_data_processing" {
  user_id   = unionai_user.admin.id
  policy_id = unionai_policy.ml_training_admin.id
}

# Data scientist has developer access to ML training
resource "unionai_user_access" "ds_ml_training" {
  user_id   = unionai_user.data_scientist.id
  policy_id = unionai_policy.ml_training_admin.id
}

# ML engineer has developer access to data processing
resource "unionai_user_access" "mle_data_processing" {
  user_id   = unionai_user.ml_engineer.id
  policy_id = unionai_policy.data_processing_developer.id
}

# Both can view production
resource "unionai_user_access" "ds_production" {
  user_id   = unionai_user.data_scientist.id
  policy_id = unionai_policy.production_viewer.id
}

resource "unionai_user_access" "mle_production" {
  user_id   = unionai_user.ml_engineer.id
  policy_id = unionai_policy.production_viewer.id
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
  user_id   = data.unionai_user.existing_user.id
  policy_id = unionai_policy.production_viewer.id
}
```

## Best Practices

1. **Use Variables**: Define user emails and project names as variables for easier management
2. **Organize by Environment**: Use Terraform workspaces or separate directories for dev/staging/prod
3. **Version Control**: Keep your Terraform configuration in Git
4. **State Management**: Use remote state (S3, Terraform Cloud) for team collaboration
5. **Security**: Never commit API keys or sensitive data - use variables and `.tfvars` files

## Complete Example

See the [examples/resources](../../examples/resources/) directory for a complete working example.
