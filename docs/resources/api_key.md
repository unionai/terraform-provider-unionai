---
page_title: "unionai_api_key Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages a Union.ai API key.
---

# unionai_api_key (Resource)

Manages a Union.ai API key. API keys are used for authentication with the Union.ai API, particularly for non-interactive use cases like CI/CD pipelines.

**Note:** The `id` attribute cannot be changed after creation. Changing it will force replacement of the resource.

## Example Usage

```terraform
resource "unionai_api_key" "ci_cd" {
  id = "ci-cd-pipeline-key"
}

# Output the secret (be careful with this in production!)
output "api_key_secret" {
  value     = unionai_api_key.ci_cd.secret
  sensitive = true
}
```

## Schema

### Required

- `id` (String) The identifier for the API key. This must be unique within your organization. Changing this forces a new resource to be created.

### Read-Only

- `secret` (String, Sensitive) The API key secret. This is only available after creation and is stored in the Terraform state. Handle this value securely.

## Important Notes

- The `secret` attribute contains sensitive credentials. Ensure your Terraform state is stored securely.
- The API key secret is only computed once during creation. If you lose access to the state file, you will need to create a new API key.
- API keys created through this resource can be used with the Union CLI and API.

## Import

API keys can be imported using their ID, but note that the secret will not be available after import:

```shell
terraform import unionai_api_key.example api-key-id
```
