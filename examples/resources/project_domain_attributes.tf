resource "unionai_project" "test" {
  name        = "test"
  description = "Test Project"
}

# Bind a per-project IAM role to the project-domain namespace's default
# ServiceAccount by setting the defaultIamRole cluster resource template variable.
resource "unionai_project_domain_attributes" "test" {
  project = unionai_project.test.id
  domain  = "development"

  attributes = {
    defaultIamRole = "arn:aws:iam::123456789012:role/my-project-development-role"
  }
}
