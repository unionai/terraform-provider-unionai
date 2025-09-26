resource "unionai_user" "nelson" {
  first_name = "Nelson"
  last_name  = "Araujo"
  email      = "nelson@union.ai"
}

resource "unionai_project" "nelson" {
  name        = "nelson"
  description = "Nelson's Playground"
}

# Uses an existing role
resource "unionai_role" "admin" {
  name = "admin"
  actions = [
    "administer_account",
    "administer_project",
  ]
}

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

resource "unionai_policy_binding" "nelson" {
  user   = unionai_user.nelson.id
  policy = unionai_policy.project1_admins.id
}
