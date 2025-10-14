resource "unionai_application_access" "example" {
  app    = "tryv2-uctl"
  policy = "contributor"
}

output "application_access" {
  value = unionai_application_access.example
}
