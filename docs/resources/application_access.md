---
page_title: "unionai_app_access Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages access policies for a Union.ai application.
---

# unionai_app_access (Resource)

Manages access policies for a Union.ai application. This resource assigns policies to applications, granting them specific permissions.

## Example Usage

```terraform
resource "unionai_application" "ci_app" {
  client_id   = "ci-pipeline"
  client_name = "CI/CD Pipeline"

  grant_types = ["CLIENT_CREDENTIALS"]
  response_types = ["CODE"]
}

resource "unionai_policy" "ci_access" {
  name        = "ci-access"
  description = "Access for CI/CD pipelines"

  project {
    id      = unionai_project.example.id
    role_id = unionai_role.ci_executor.id
  }
}

resource "unionai_app_access" "ci_permissions" {
  app_id    = unionai_application.ci_app.id
  policy_id = unionai_policy.ci_access.id
}
```

## Schema

### Required

- `app_id` (String) The ID of the application to grant access to.
- `policy_id` (String) The ID of the policy to assign to the application.

### Read-Only

- `id` (String) The unique identifier of the application access assignment.

## Import

Application access assignments can be imported using their ID:

```shell
terraform import unionai_app_access.example app-access-id
```
