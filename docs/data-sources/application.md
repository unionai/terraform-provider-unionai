---
page_title: "unionai_application Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about a Union.ai OAuth application.
---

# unionai_application (Data Source)

Retrieves information about a Union.ai OAuth application.

**Note:** The secret is not available through the data source for security reasons. Secrets are only available when creating the resource.

## Example Usage

```terraform
data "unionai_application" "example" {
  id = "app-client-id"
}

output "app_name" {
  value = data.unionai_application.example.client_name
}
```

## Schema

### Required

- `id` (String) The unique identifier (client ID) of the application.

### Read-Only

- `client_id` (String) The OAuth client ID.
- `client_name` (String) Human-readable name of the application.
- `client_uri` (String) URI of the application's website.
- `consent_method` (String) The consent method used by the application.
- `grant_types` (Set of String) List of OAuth 2.0 grant types the application may use.
- `logo_uri` (String) URI that references a logo for the application.
- `policy_uri` (String) URI for the application's privacy policy.
- `redirect_uris` (Set of String) List of valid redirect URIs for OAuth callbacks.
- `response_types` (Set of String) List of OAuth 2.0 response types.
- `token_endpoint_auth_method` (String) Authentication method for the token endpoint.
- `tos_uri` (String) URI for the application's terms of service.
- `secret` (String, Sensitive) The OAuth client secret. Note: This will be empty when reading an existing application.
