---
page_title: "Access Control with Policies"
subcategory: "Common Scenarios"
---

# Access Control with Policies

This guide explains how to implement fine-grained access control in Union.ai using Terraform policies, roles, and user access assignments.

## Understanding Union.ai Access Control

Union.ai uses a role-based access control (RBAC) model with three key components:

1. **Roles**: Define sets of permissions. Union.ai provides three built-in roles:
   - **admin**: Complete administrative access
   - **contributor**: Can create and modify workflows
   - **viewer**: Read-only access to workflows and executions
2. **Policies**: Bind roles to specific projects and domains
3. **User Access**: Assign policies to users or applications

## Scenario: Multi-Tier Access Control

Let's set up a realistic access control structure for a data science team.

### Understanding Project Domains

Each Union.ai project includes three built-in domains:

- **development**: For active development and experimentation
- **staging**: For pre-production testing and validation
- **production**: For production workflows

These domains are automatically available in every project, so you don't need to create separate projects for different environments.

### Step 1: Create Your Project

```terraform
resource "unionai_project" "ml_workflows" {
  name        = "ml-workflows"
  description = "Machine learning workflow project"
}
```

### Step 2: Choose Your Roles

You have two options for roles: use built-in roles or create custom roles.

#### Option A: Use Built-in Roles (Recommended)

Union.ai provides three built-in roles that cover most use cases. Load them using data sources:

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

**When to use built-in roles:**
- You need standard permission levels (full admin, read/write, read-only)
- You want to keep your configuration simple
- Your team follows common access patterns

#### Option B: Create Custom Roles

If you need fine-grained control over permissions, create custom roles:

```terraform
resource "unionai_role" "data_scientist" {
  name        = "data-scientist"
  description = "Custom role for data scientists with specific workflow permissions"
  actions = [
    "view_flyte_executions",
    "create_flyte_executions",
    "view_flyte_inventory",
    "write_flyte_inventory",
  ]
}

resource "unionai_role" "workflow_viewer" {
  name        = "workflow-viewer"
  description = "Can view workflows but not execute them"
  actions = [
    "view_flyte_executions",
    "view_flyte_inventory",
  ]
}
```

**When to use custom roles:**
- You need specific combinations of permissions
- Your security requirements demand least-privilege access
- You want to limit access to specific actions

**Note:** The rest of this guide uses built-in roles for simplicity, but you can substitute custom roles anywhere a role is referenced.

### Step 3: Create Domain-Specific Policies

Create policies for different roles across the project's built-in domains:

```terraform
# Development domain policies - more permissive
resource "unionai_policy" "dev_admin" {
  name = "ml-workflows-dev-admin"

  project {
    id      = unionai_project.ml_workflows.id
    role_id = data.unionai_role.admin.id
    domains = ["development"]
  }
}

resource "unionai_policy" "dev_contributor" {
  name = "ml-workflows-dev-contributor"

  project {
    id      = unionai_project.ml_workflows.id
    role_id = data.unionai_role.contributor.id
    domains = ["development"]
  }
}

# Staging domain policies - moderate restrictions
resource "unionai_policy" "staging_contributor" {
  name = "ml-workflows-staging-contributor"

  project {
    id      = unionai_project.ml_workflows.id
    role_id = data.unionai_role.contributor.id
    domains = ["staging"]
  }
}

resource "unionai_policy" "staging_viewer" {
  name = "ml-workflows-staging-viewer"

  project {
    id      = unionai_project.ml_workflows.id
    role_id = data.unionai_role.viewer.id
    domains = ["staging"]
  }
}

# Production domain policies - most restrictive
resource "unionai_policy" "prod_contributor" {
  name = "ml-workflows-prod-contributor"

  project {
    id      = unionai_project.ml_workflows.id
    role_id = data.unionai_role.contributor.id
    domains = ["production"]
  }
}

resource "unionai_policy" "prod_viewer" {
  name = "ml-workflows-prod-viewer"

  project {
    id      = unionai_project.ml_workflows.id
    role_id = data.unionai_role.viewer.id
    domains = ["production"]
  }
}
```

### Step 4: Assign Users Based on Their Roles

```terraform
# Senior engineers get admin in dev, contributor in staging and prod
resource "unionai_user_access" "senior_dev" {
  user   = unionai_user.senior_engineer.id
  policy = unionai_policy.dev_admin.id
}

resource "unionai_user_access" "senior_staging" {
  user   = unionai_user.senior_engineer.id
  policy = unionai_policy.staging_contributor.id
}

resource "unionai_user_access" "senior_prod" {
  user   = unionai_user.senior_engineer.id
  policy = unionai_policy.prod_contributor.id
}

# Junior engineers get contributor in dev, viewer in staging and prod
resource "unionai_user_access" "junior_dev" {
  user   = unionai_user.junior_engineer.id
  policy = unionai_policy.dev_contributor.id
}

resource "unionai_user_access" "junior_staging" {
  user   = unionai_user.junior_engineer.id
  policy = unionai_policy.staging_viewer.id
}

resource "unionai_user_access" "junior_prod" {
  user   = unionai_user.junior_engineer.id
  policy = unionai_policy.prod_viewer.id
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

# Create a policy for automated deployments to staging domain
resource "unionai_policy" "automated_deployment" {
  name = "ci-cd-staging-deployment"

  project {
    id      = unionai_project.ml_workflows.id
    role_id = data.unionai_role.contributor.id
    domains = ["staging"]
  }
}

# Grant the application access
resource "unionai_application_access" "ci_cd_staging" {
  application = unionai_application.ci_cd_bot.id
  policy      = unionai_policy.automated_deployment.id
}
```

## Policy Scoping Options

Policies can be scoped at different levels using optional blocks. You can use either built-in roles or custom roles in any of these scoping options:

#### Organization-Level Access

Grant access across the entire organization:

```terraform
resource "unionai_policy" "org_admin" {
  name = "organization-admin-policy"

  organization {
    id      = "your-org-id"
    role_id = data.unionai_role.admin.id
  }
}
```

#### Domain-Level Access

Grant access to all projects within a specific domain:

```terraform
resource "unionai_policy" "staging_reviewers" {
  name = "staging-reviewers-policy"

  domain {
    id      = "staging"
    role_id = data.unionai_role.viewer.id
  }
}
```

#### Project-Level Access

Grant access to specific projects and domains (most common):

```terraform
# Using a built-in role
resource "unionai_policy" "project_contributors" {
  name = "project-contributors-policy"

  project {
    id      = unionai_project.ml_workflows.id
    role_id = data.unionai_role.contributor.id
    domains = ["development", "staging", "production"]
  }
}

# Using a custom role
resource "unionai_policy" "project_data_scientists" {
  name = "project-data-scientists-policy"

  project {
    id      = unionai_project.ml_workflows.id
    role_id = unionai_role.data_scientist.id  # custom role
    domains = ["development", "staging"]
  }
}
```

You can also combine multiple scoping blocks in a single policy:

```terraform
resource "unionai_policy" "multi_scope" {
  name = "multi-scope-policy"

  # Access to entire organization
  organization {
    id      = "your-org-id"
    role_id = data.unionai_role.admin.id
  }

  # Access to all development domains
  domain {
    id      = "development"
    role_id = data.unionai_role.contributor.id
  }

  # Access to specific project
  project {
    id      = unionai_project.ml_workflows.id
    role_id = data.unionai_role.viewer.id
    domains = ["production"]
  }
}
```

## Dynamic Access with Variables

Use variables to make access control configurable:

```terraform
variable "team_members" {
  description = "Map of team members and their access levels"
  type = map(object({
    email  = string
    name   = string
    policy_ids = list(string)
  }))
  default = {
    "alice" = {
      email      = "alice@example.com"
      name       = "Alice Smith"
      policy_ids = ["dev-admin", "staging-contributor", "prod-viewer"]
    }
    "bob" = {
      email      = "bob@example.com"
      name       = "Bob Jones"
      policy_ids = ["dev-contributor", "staging-viewer"]
    }
  }
}

# Create users dynamically
resource "unionai_user" "team" {
  for_each = var.team_members

  email = each.value.email
  name  = each.value.name
}

# Assign access dynamically
resource "unionai_user_access" "team_access" {
  for_each = {
    for pair in flatten([
      for user_key, user in var.team_members : [
        for policy_id in user.policy_ids : {
          key       = "${user_key}-${policy_id}"
          user_id   = user_key
          policy_id = policy_id
        }
      ]
    ]) : pair.key => pair
  }

  user   = unionai_user.team[each.value.user_id].id
  policy = data.unionai_policy[each.value.policy_id].id
}
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
2. **Domain Isolation**: Use stricter policies in production domains than in development or staging
3. **Leverage Built-in Domains**: Use the three built-in domains (development, staging, production) within each project rather than creating separate projects for each environment
4. **Regular Reviews**: Periodically review and update access policies
5. **Automation**: Use Terraform to ensure consistent access control across domains
6. **Documentation**: Comment your policies to explain why specific access was granted

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
