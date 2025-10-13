resource "unionai_application" "myapp" {
  client_id      = "dummy-client-name"
  client_name    = "dummy client name"
  client_uri     = "https://dummyclienturi"
  consent_method = "CONSENT_METHOD_REQUIRED"
  contacts = [
    "dummy@dummy.com"
  ]
  grant_types = [
    "AUTHORIZATION_CODE",
  ]
  jwks_uri   = "https://dummyjwksuri"
  logo_uri   = "https://logouri"
  policy_uri = "https://dummypolicyuri"
  redirect_uris = [
    "https://dummy/callback"
  ]
  response_types = [
    "CODE",
  ]
  token_endpoint_auth_method = "CLIENT_SECRET_POST"
  tos_uri                    = "https://dummytosuri"
}

output "oauth_app_secret_id" {
  value = unionai_oauth_app.myapp.id
}

output "oauth_app_secret" {
  sensitive = true
  value     = unionai_application.myapp.secret
}
