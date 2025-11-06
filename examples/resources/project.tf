resource "unionai_project" "test" {
  name        = "test"
  description = "Test Project"
}

output "project" {
  value = unionai_project.test
}
