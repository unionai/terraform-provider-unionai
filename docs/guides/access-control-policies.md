---
page_title: "Access Control with Policies"
subcategory: "Common Scenarios"
---

# Access Control with Policies

This guide explains how to implement fine-grained access control in Union.ai using Terraform policies, roles, and user access assignments.

## Understanding Union.ai Access Control

Union.ai uses a role-based access control (RBAC) model with three key components:

1. **Roles**: Define sets of permissions
2. **Policies**: Bind roles to specific projects or resources
3. **User Access**: Assign policies to users or applications

## Scenario: Multi-Tier Access Control

Let's set up a realistic access control structure for a data science team.

### Step 1: Create Projects by Environment

```terraform
resource "unionai_project" "dev" {
  name        = "dev-environment"
  description = "Development and experimentation"
}

resource "unionai_project" "staging" {
  name        = "staging-environment"
  description = "Pre-production testing"
}

resource "unionai_project" "prod" {
  name        = "production"
  description = "Production workflows"
}
```

### Step 2: Define Roles with Different Permission Levels

```terraform
resource "unionai_role" "full_admin" {
  name        = "full-admin"
  description = "Complete administrative access"
}

resource "unionai_role" "workflow_developer" {
  name        = "workflow-developer"
  description = "Can create and modify workflows"
}

resource "unionai_role" "workflow_executor" {
  name        = "workflow-executor"
  description = "Can execute workflows but not modify them"
}

resource "unionai_role" "read_only" {
  name        = "read-only"
  description = "View-only access to workflows and executions"
}
```

### Step 3: Create Environment-Specific Policies

```terraform
# Development policies - more permissive
resource "unionai_policy" "dev_admin" {
  project_id = unionai_project.dev.id
  role_id    = unionai_role.full_admin.id
}

resource "unionai_policy" "dev_developer" {
  project_id = unionai_project.dev.id
  role_id    = unionai_role.workflow_developer.id
}

# Staging policies - moderate restrictions
resource "unionai_policy" "staging_developer" {
  project_id = unionai_project.staging.id
  role_id    = unionai_role.workflow_developer.id
}

resource "unionai_policy" "staging_executor" {
  project_id = unionai_project.staging.id
  role_id    = unionai_role.workflow_executor.id
}

# Production policies - most restrictive
resource "unionai_policy" "prod_executor" {
  project_id = unionai_project.prod.id
  role_id    = unionai_role.workflow_executor.id
}

resource "unionai_policy" "prod_viewer" {
  project_id = unionai_project.prod.id
  role_id    = unionai_role.read_only.id
}
```

### Step 4: Assign Users Based on Their Roles

```terraform
# Senior engineers get admin in dev, developer in staging, executor in prod
resource "unionai_user_access" "senior_dev" {
  user_id   = unionai_user.senior_engineer.id
  policy_id = unionai_policy.dev_admin.id
}

resource "unionai_user_access" "senior_staging" {
  user_id   = unionai_user.senior_engineer.id
  policy_id = unionai_policy.staging_developer.id
}

resource "unionai_user_access" "senior_prod" {
  user_id   = unionai_user.senior_engineer.id
  policy_id = unionai_policy.prod_executor.id
}

# Junior engineers get developer in dev, viewer in staging
resource "unionai_user_access" "junior_dev" {
  user_id   = unionai_user.junior_engineer.id
  policy_id = unionai_policy.dev_developer.id
}

resource "unionai_user_access" "junior_staging" {
  user_id   = unionai_user.junior_engineer.id
  policy_id = unionai_policy.staging_executor.id
}

resource "unionai_user_access" "junior_prod" {
  user_id   = unionai_user.junior_engineer.id
  policy_id = unionai_policy.prod_viewer.id
}
```

## Application Access Control

You can also grant access to applications (service accounts):

```terraform
# Create an application for CI/CD
resource "unionai_application" "ci_cd_bot" {
  name        = "ci-cd-deployment"
  description = "Automated deployment application"
}

# Create a policy for automated deployments
resource "unionai_policy" "automated_deployment" {
  project_id = unionai_project.staging.id
  role_id    = unionai_role.workflow_executor.id
}

# Grant the application access
resource "unionai_application_access" "ci_cd_staging" {
  application_id = unionai_application.ci_cd_bot.id
  policy_id      = unionai_policy.automated_deployment.id
}
```

## Dynamic Access with Variables

Use variables to make access control configurable:

```terraform
variable "team_members" {
  description = "Map of team members and their access levels"
  type = map(object({
    email       = string
    name        = string
    dev_role    = string
    staging_role = string
    prod_role   = string
  }))
}

# Example usage in terraform.tfvars:
# team_members = {
#   "alice" = {
#     email        = "alice@example.com"
#     name         = "Alice Smith"
#     dev_role     = "full-admin"
#     staging_role = "workflow-developer"
#     prod_role    = "workflow-executor"
#   }
# }
```

## Audit and Compliance

Use Terraform state to maintain an audit trail:

```terraform
output "access_matrix" {
  description = "Summary of all access assignments"
  value = {
    users = {
      for user in unionai_user.* :
      user.email => {
        id      = user.id
        name    = user.name
        access  = [for access in unionai_user_access.* : access if access.user_id == user.id]
      }
    }
  }
}
```

## Best Practices

1. **Principle of Least Privilege**: Start with minimal permissions and grant more as needed
2. **Environment Isolation**: Use stricter policies in production environments
3. **Regular Reviews**: Periodically review and update access policies
4. **Automation**: Use Terraform to ensure consistent access control across environments
5. **Documentation**: Comment your policies to explain why specific access was granted

## Removing Access

To revoke access, simply remove the corresponding `unionai_user_access` or `unionai_application_access` resource and apply:

```bash
terraform plan
terraform apply
```

## Related Resources

- [unionai_role](/docs/resources/unionai_role.md) - Role resource documentation
- [unionai_policy](/docs/resources/unionai_policy.md) - Policy resource documentation
- [unionai_user_access](/docs/resources/unionai_user_access.md) - User access resource documentation
