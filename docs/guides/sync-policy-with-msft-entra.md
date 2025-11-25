---
page_title: "Synchronizing a Policy with Microsoft Entra ID Group"
subcategory: "Identity Provider Sync"
---

# Synchronizing a Policy with Microsoft Entra ID Group

This guide demonstrates how to create a project and policy in Union.ai, then automatically grant access to all users in a Microsoft Entra ID (formerly Azure Active Directory) group.

## Overview

Instead of manually assigning individual users to policies, you can synchronize a Union.ai policy with a Microsoft Entra ID group. When users are added to or removed from the Entra ID group, their access to Union.ai is automatically updated.

## Prerequisites

- Access to Microsoft Entra ID admin center
- Union.ai organization configured with Microsoft Entra ID SSO
- Terraform installed and configured with Union.ai provider

## Use Case: ML Engineering Team

Let's set up access for an ML engineering team:

1. Create a project called "ml-experiments"
2. Create a policy called "ml-engineers" with contributor access
3. Grant access to all members of the Microsoft Entra ID group "ML Engineers"

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

### Step 4: Read Microsoft Entra ID Group Members

Use the AzureAD provider to read your existing Microsoft Entra ID group and its members:

```terraform
# Reference existing Microsoft Entra ID group
data "azuread_group" "ml_engineers" {
  display_name     = "ML Engineers"
  security_enabled = true
}

# Read all members of the group
data "azuread_group_members" "ml_engineers_members" {
  group_object_id = data.azuread_group.ml_engineers.object_id
}
```

### Step 5: Get User Details

Fetch detailed user information for each group member:

```terraform
data "azuread_user" "ml_engineers" {
  for_each = toset(data.azuread_group_members.ml_engineers_members.members)

  object_id = each.value
}
```

### Step 6: Create Union.ai Users

Create Union.ai users with first and last names from Entra ID:

```terraform
resource "unionai_user" "ml_engineers" {
  for_each = data.azuread_user.ml_engineers

  email      = each.value.user_principal_name
  first_name = coalesce(each.value.given_name, split("@", each.value.user_principal_name)[0])
  last_name  = coalesce(each.value.surname, "")
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

**Important Notes:**
- The group must already exist in Microsoft Entra ID
- The `display_name` must exactly match the Microsoft Entra ID group Display Name
- You'll need to configure the AzureAD Terraform provider with appropriate credentials

### Step 8: Apply the Configuration

```bash
terraform init
terraform plan
terraform apply
```

## How It Works

1. **Group Member Enumeration**: The AzureAD provider reads all member object IDs from the specified Entra ID group
2. **User Details Fetch**: For each member, fetch their full user details including first and last name using `azuread_user`
3. **User Creation**: Union.ai user accounts are created for each group member using `for_each`
4. **Access Assignment**: Each user is granted access to the project via the policy
5. **Terraform State Management**: When users are added to or removed from the Entra ID group, running `terraform apply` will update Union.ai accordingly

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

# Reference existing Microsoft Entra ID group
data "azuread_group" "ml_engineers" {
  display_name     = "ML Engineers"
  security_enabled = true
}

# Read all members of the group
data "azuread_group_members" "ml_engineers_members" {
  group_object_id = data.azuread_group.ml_engineers.object_id
}

# Get user details for each member
data "azuread_user" "ml_engineers" {
  for_each = toset(data.azuread_group_members.ml_engineers_members.members)

  object_id = each.value
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

# Create Union.ai users from Entra ID group members
resource "unionai_user" "ml_engineers" {
  for_each = data.azuread_user.ml_engineers

  email      = each.value.user_principal_name
  first_name = coalesce(each.value.given_name, split("@", each.value.user_principal_name)[0])
  last_name  = coalesce(each.value.surname, "")
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

1. **Check Entra ID Group Membership**: Verify users are in the correct Microsoft Entra ID group
2. **Terraform Apply**: Run `terraform apply` to create Union.ai users and grant them access
3. **User Login**: Have users log into Union.ai
4. **Access Verification**: Users should have access to the project based on their assigned policies

## Finding Your Group Name

To find the exact group name in Microsoft Entra ID:

1. Go to [Microsoft Entra admin center](https://entra.microsoft.com/)
2. Navigate to **Groups** > **All groups**
3. Find your group and click on it
4. Use the **Display name** field value in your Terraform configuration

## Troubleshooting

### Users Don't Have Access After Login

- **Verify user creation**: Check that users were created with `terraform state list | grep unionai_user`
- **Check user access**: Verify access assignments with `terraform state list | grep unionai_user_access`
- **Group membership**: Verify the user is actually a member of the Entra ID group
- **Terraform apply**: Make sure you ran `terraform apply` after adding users to the Entra ID group

### AzureAD Provider Issues

- **Authentication**: Verify your Azure AD service principal credentials are correct
- **API permissions**: Ensure the service principal has `GroupMember.Read.All` and `User.Read.All` permissions
- **Tenant configuration**: Check that `tenant_id` and `client_id` are correct
- **Terraform state**: Run `terraform plan` to check if resources are correctly configured

## Best Practices

1. **Group Naming**: Use clear, descriptive names for your Microsoft Entra ID groups that match their purpose
2. **Security Groups**: Use Security groups rather than Microsoft 365 groups for access control
3. **Automated Updates**: Set up CI/CD to run `terraform apply` regularly to sync group membership changes
4. **Domain Separation**: Use different policies for different domains (dev/staging/prod) to implement proper access control
5. **Documentation**: Document which Entra ID groups map to which Union.ai policies
6. **Regular Audits**: Periodically review Entra ID group memberships and Terraform state to ensure proper access control

## Security Considerations

- **Least Privilege**: Start with minimal permissions and grant more as needed
- **Production Access**: Use stricter policies for production domains
- **Group Management**: Limit who can add/remove users from Entra ID groups using role-based access control
- **Service Principal Security**: Protect Azure AD service principal credentials
- **Terraform State**: Secure your Terraform state as it contains user information
- **Conditional Access**: Consider using Microsoft's Conditional Access policies for additional security

## Related Resources

- [Access Control with Policies](/docs/guides/access-control-policies.md)
- [Managing Projects and Users](/docs/guides/managing-projects-users.md)
- [Synchronizing with Google Workspace](/docs/guides/idp_sync/sync-policy-with-goog-ad.md)
- [unionai_user resource](/docs/resources/unionai_user.md)
- [unionai_user_access resource](/docs/resources/unionai_user_access.md)
