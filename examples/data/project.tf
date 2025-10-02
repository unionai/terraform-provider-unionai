data "unionai_project" "nelson" {
  id = "nelson"
}

output "project" {
  value = data.unionai_project.nelson
}
