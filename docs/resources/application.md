---
page_title: "unionai_application Resource - terraform-provider-unionai"
subcategory: ""
description: |-
  Manages a Union.ai OAuth application.
---

# unionai_application (Resource)

Manages a Union.ai OAuth application. Applications are OAuth 2.0 clients that can authenticate with Union.ai.

**Note:** The `client_id` attribute cannot be changed after creation. Changing it will force replacement of the resource.

## Example Usage

```terraform
resource "unionai_application" "web_app" {
  client_id   = "my-web-app"
  client_name = "My Web Application"
  client_uri  = "https://example.com"
  logo_uri    = "https://example.com/logo.png"
  policy_uri  = "https://example.com/privacy"
  tos_uri     = "https://example.com/terms"

  consent_method           = "CONSENT_METHOD_REQUIRED"
  token_endpoint_auth_method = "CLIENT_SECRET_BASIC"

  grant_types = [
    "CLIENT_CREDENTIALS",
    "AUTHORIZATION_CODE"
  ]

  response_types = [
    "CODE"
  ]

  redirect_uris = [
    "https://example.com/callback",
    "http://localhost:8080/callback"
  ]
}

output "app_secret" {
  value     = unionai_application.web_app.secret
  sensitive = true
}
```

## Schema

### Required

- `client_id` (String) The OAuth client ID for the application. This must be unique within your organization. Changing this forces a new resource to be created.
- `client_name` (String) Human-readable name of the application.

### Optional

- `client_uri` (String) URI of the application's website.
- `consent_method` (String) The consent method used by the application. Common values include `CONSENT_METHOD_REQUIRED`.
- `grant_types` (Set of String) List of OAuth 2.0 grant types the application may use. Common values: `CLIENT_CREDENTIALS`, `AUTHORIZATION_CODE`, `REFRESH_TOKEN`.
- `logo_uri` (String) URI that references a logo for the application.
- `policy_uri` (String) URI that the application provides to end-users to read about how their profile data will be used.
- `redirect_uris` (Set of String) List of valid redirect URIs for OAuth callbacks.
- `response_types` (Set of String) List of OAuth 2.0 response types the application may use. Common values: `CODE`, `TOKEN`.
- `token_endpoint_auth_method` (String) Authentication method for the token endpoint. Common values: `CLIENT_SECRET_BASIC`, `CLIENT_SECRET_POST`.
- `tos_uri` (String) URI that the application provides to end-users for terms of service.

### Read-Only

- `id` (String) The unique identifier of the application.
- `secret` (String, Sensitive) The OAuth client secret. This is only available after creation and is stored in the Terraform state. Handle this value securely.

## Important Notes

- The `secret` attribute contains sensitive OAuth credentials. Ensure your Terraform state is stored securely.
- The application secret is only computed once during creation. If you lose access to the state file, you will need to create a new application.

## Import

Applications can be imported using their client ID, but note that the secret will not be available after import:

```shell
terraform import unionai_application.example client-id
```
