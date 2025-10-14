resource "unionai_policy" "project1_admins" {
  name = "project-1-admins"

  role {
    id = unionai_role.admin.id
    resource {
      project = unionai_project.nelson.name
      domain  = "development"
    }
  }

  role {
    id = unionai_role.admin.id
    resource {
      project = unionai_project.nelson.name
      domain  = "staging"
    }
  }
}

output "project1_admins" {
  value = unionai_policy.project1_admins
}
