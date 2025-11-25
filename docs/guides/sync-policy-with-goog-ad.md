---
page_title: "Synchronizing a Policy with Google Domain Group"
subcategory: "Identity Provider Sync"
---

# Synchronizing a Policy with Google Domain Group

This guide demonstrates how to create a project and policy in Union.ai, then automatically grant access to all users in a Google Workspace (formerly Google Apps/G Suite) group.

## Overview

Instead of manually assigning individual users to policies, you can synchronize a Union.ai policy with a Google Workspace group. When users are added to or removed from the Google group, their access to Union.ai is automatically updated.

## Prerequisites

- Access to Google Workspace admin console
- Union.ai organization configured with Google Workspace SSO
- Terraform installed and configured with Union.ai provider

## Use Case: ML Engineering Team

Let's set up access for an ML engineering team:

1. Create a project called "ml-experiments"
2. Create a policy called "ml-engineers" with contributor access
3. Grant access to all members of the Google group "ML Engineers"

## Step-by-Step Implementation

### Step 1: Create the Project

Create a project for ML experimentation. Each project automatically includes development, staging, and production domains.

```terraform
resource "unionai_project" "ml_experiments" {
  name        = "ml-experiments"
  description = "Machine learning experimentation project"
}
```

### Step 2: Reference Built-in Roles

Load the built-in contributor role for the ML engineers:

```terraform
data "unionai_role" "contributor" {
  name = "contributor"
}
```

### Step 3: Create a Policy for ML Engineers

Create a policy that grants contributor access to the development and staging domains:

```terraform
resource "unionai_policy" "ml_engineers" {
  name = "ml-engineers"

  project {
    id      = unionai_project.ml_experiments.id
    role_id = data.unionai_role.contributor.id
    domains = ["development", "staging"]
  }
}
```

### Step 4: Read Google Group Members

Use the Google Workspace provider to read group members:

```terraform
# Reference existing Google Workspace group
data "googleworkspace_group" "ml_engineers" {
  email = "ml-engineers@yourdomain.com"
}

# Read all members of the group
data "googleworkspace_group_members" "ml_engineers_members" {
  group_id = data.googleworkspace_group.ml_engineers.id
}
```

### Step 5: Get User Details

Fetch detailed user information for each group member:

```terraform
data "googleworkspace_user" "ml_engineers" {
  for_each = {
    for member in data.googleworkspace_group_members.ml_engineers_members.members :
    member.email => member
  }

  primary_email = each.value.email
}
```

### Step 6: Create Union.ai Users

Create Union.ai users with first and last names from Google Workspace:

```terraform
resource "unionai_user" "ml_engineers" {
  for_each = data.googleworkspace_user.ml_engineers

  email      = each.value.primary_email
  first_name = each.value.name[0].given_name
  last_name  = each.value.name[0].family_name
}
```

### Step 7: Grant Access to Users

Grant access to each user using the policy:

```terraform
resource "unionai_user_access" "ml_engineers" {
  for_each = unionai_user.ml_engineers

  user   = each.value.id
  policy = unionai_policy.ml_engineers.id
}
```

### Step 8: Apply the Configuration

```bash
terraform init
terraform plan
terraform apply
```

## How It Works

1. **Group Member Enumeration**: The Google Workspace provider reads all members from the specified Google group
2. **User Details Fetch**: For each group member, fetch their full user details including first and last name
3. **User Creation**: Union.ai user accounts are created for each group member using `for_each`
4. **Access Assignment**: Each user is granted access to the project via the policy
5. **Terraform State Management**: When users are added to or removed from the Google group, running `terraform apply` will update Union.ai accordingly

## Complete Example

Here's a complete Terraform configuration:

```terraform
# Create the project
resource "unionai_project" "ml_experiments" {
  name        = "ml-experiments"
  description = "Machine learning experimentation project"
}

# Reference built-in roles
data "unionai_role" "contributor" {
  name = "contributor"
}

data "unionai_role" "viewer" {
  name = "viewer"
}

# Reference existing Google Workspace group
data "googleworkspace_group" "ml_engineers" {
  email = "ml-engineers@yourdomain.com"
}

# Read all members of the group
data "googleworkspace_group_members" "ml_engineers_members" {
  group_id = data.googleworkspace_group.ml_engineers.id
}

# Get user details for each member
data "googleworkspace_user" "ml_engineers" {
  for_each = {
    for member in data.googleworkspace_group_members.ml_engineers_members.members :
    member.email => member
  }

  primary_email = each.value.email
}

# Create policies for different access levels
resource "unionai_policy" "ml_engineers" {
  name = "ml-engineers"

  project {
    id      = unionai_project.ml_experiments.id
    role_id = data.unionai_role.contributor.id
    domains = ["development", "staging"]
  }
}

resource "unionai_policy" "ml_engineers_prod_viewer" {
  name = "ml-engineers-prod-viewer"

  project {
    id      = unionai_project.ml_experiments.id
    role_id = data.unionai_role.viewer.id
    domains = ["production"]
  }
}

# Create Union.ai users from Google group members
resource "unionai_user" "ml_engineers" {
  for_each = data.googleworkspace_user.ml_engineers

  email      = each.value.primary_email
  first_name = each.value.name[0].given_name
  last_name  = each.value.name[0].family_name
}

# Grant access for dev/staging
resource "unionai_user_access" "ml_engineers_dev_staging" {
  for_each = unionai_user.ml_engineers

  user   = each.value.id
  policy = unionai_policy.ml_engineers.id
}

# Grant viewer access for production
resource "unionai_user_access" "ml_engineers_prod" {
  for_each = unionai_user.ml_engineers

  user   = each.value.id
  policy = unionai_policy.ml_engineers_prod_viewer.id
}

# Output the configuration
output "project_info" {
  value = {
    project_id   = unionai_project.ml_experiments.id
    project_name = unionai_project.ml_experiments.name
  }
}

output "users_created" {
  value = {
    for email, user in unionai_user.ml_engineers :
    email => user.id
  }
}

output "policy_info" {
  value = {
    ml_engineers_policy_id      = unionai_policy.ml_engineers.id
    ml_engineers_prod_policy_id = unionai_policy.ml_engineers_prod_viewer.id
  }
}
```

## Benefits of This Approach

**Advantages:**
- Explicit control over each user in Terraform state
- Can see all users and their access assignments
- Can grant different users from the same group access to different policies
- Works with or without SSO
- When group membership changes, running `terraform apply` syncs the changes

**Considerations:**
- Requires running `terraform apply` to sync changes (not automatic on login)
- More verbose configuration
- All users are tracked in Terraform state

## Verifying Access

After applying your Terraform configuration:

1. **Check Google Group Membership**: Verify users are in the correct Google Workspace group
2. **Terraform Apply**: Run `terraform apply` to create Union.ai users and grant them access
3. **User Login**: Have users log into Union.ai
4. **Access Verification**: Users should have access to the project based on their assigned policies

## Troubleshooting

### Users Don't Have Access After Login

- **Verify user creation**: Check that users were created with `terraform state list | grep unionai_user`
- **Check user access**: Verify access assignments with `terraform state list | grep unionai_user_access`
- **Group membership**: Verify the user is actually a member of the Google group
- **Terraform apply**: Make sure you ran `terraform apply` after adding users to the Google group

### Google Workspace Provider Issues

- **Authentication**: Verify your Google Workspace service account credentials are correct
- **API permissions**: Ensure the service account has permission to read group members
- **Impersonation**: Check that `impersonated_user_email` is set to an admin account
- **Terraform state**: Run `terraform plan` to check if resources are correctly configured

## Best Practices

1. **Group Naming**: Use clear, descriptive names for your Google Workspace groups that match their purpose
2. **Automated Updates**: Set up CI/CD to run `terraform apply` regularly to sync group membership changes
3. **Domain Separation**: Use different policies for different domains (dev/staging/prod) to implement proper access control
4. **Documentation**: Document which Google groups map to which Union.ai policies
5. **Regular Audits**: Periodically review Google group memberships and Terraform state to ensure proper access control

## Security Considerations

- **Least Privilege**: Start with minimal permissions and grant more as needed
- **Production Access**: Use stricter policies for production domains
- **Group Management**: Limit who can add/remove users from Google groups
- **Service Account Security**: Protect Google Workspace service account credentials
- **Terraform State**: Secure your Terraform state as it contains user information

## Related Resources

- [Access Control with Policies](/docs/guides/access-control-policies.md)
- [Managing Projects and Users](/docs/guides/managing-projects-users.md)
- [unionai_user resource](/docs/resources/unionai_user.md)
- [unionai_user_access resource](/docs/resources/unionai_user_access.md)
