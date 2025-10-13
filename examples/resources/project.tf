resource "unionai_project" "nelson" {
  name        = "nelson"
  description = "Nelson's Playground"
}

output "project_nelson" {
  value = unionai_project.nelson
}
