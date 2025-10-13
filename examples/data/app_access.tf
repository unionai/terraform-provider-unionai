data "unionai_application_access" "example" {
  app_id    = "tryv2-uctl"
  policy_id = "contributor"
}

output "application_access" {
  value = data.unionai_application_access.example
}
