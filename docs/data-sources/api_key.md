---
page_title: "unionai_api_key Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about a Union.ai API key.
---

# unionai_api_key (Data Source)

Retrieves information about a Union.ai API key.

**Note:** The secret is not available through the data source for security reasons. Secrets are only available when creating the resource.

## Example Usage

```terraform
data "unionai_api_key" "example" {
  id = "api-key-id"
}
```

## Schema

### Required

- `id` (String) The unique identifier of the API key.

### Read-Only

- `secret` (String, Sensitive) The API key secret. Note: This will be empty when reading an existing API key. Secrets are only available during resource creation.
