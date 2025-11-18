data "unionai_policy" "viewer" {
  id = "viewer"
}

resource "unionai_policy" "some_service" {
  name = "some-service"

  // Whole organization
  organization {
    id      = "tryv2"
    role_id = unionai_role.example.id
  }

  // All projects in this domain
  domain {
    id      = "development"
    role_id = unionai_role.example.id
  }

  #// A specific project + domain(s)
  project {
    id      = unionai_project.test.id
    role_id = unionai_role.example.id
    domains = [
      "development",
    ]
  }
}

output "policy_some_service" {
  value = unionai_policy.some_service
}
