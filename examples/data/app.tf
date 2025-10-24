data "unionai_application" "myapp" {
  id = "tryv2-flyteadmin"
}

output "app" {
  value = {
    id                         = data.unionai_application.myapp.id
    client_id                  = data.unionai_application.myapp.client_id
    client_name                = data.unionai_application.myapp.client_name
    client_uri                 = data.unionai_application.myapp.client_uri
    contacts                   = data.unionai_application.myapp.contacts
    grant_types                = data.unionai_application.myapp.grant_types
    jwks_uri                   = data.unionai_application.myapp.jwks_uri
    logo_uri                   = data.unionai_application.myapp.logo_uri
    policy_uri                 = data.unionai_application.myapp.policy_uri
    redirect_uris              = data.unionai_application.myapp.redirect_uris
    response_types             = data.unionai_application.myapp.response_types
    token_endpoint_auth_method = data.unionai_application.myapp.token_endpoint_auth_method
  }
}
