package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAppAccessResource_ProjectScoped(t *testing.T) {
	suffix := fmt.Sprintf("%06d", rand.Intn(1000000))
	keyName := "tf-acc-app-access-" + suffix
	projectName := "tf-acc-project-" + suffix
	roleName := "tf-acc-role-" + suffix
	policyName := "tf-acc-policy-" + suffix

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAppAccessProjectScopedConfig(keyName, projectName, roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_application_access.test", "app", keyName),
					resource.TestCheckResourceAttr("unionai_application_access.test", "policy", policyName),
				),
			},
			// Verify the assignment exists server-side via the data source
			{
				Config: testAccAppAccessProjectScopedConfigWithDataSource(keyName, projectName, roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.unionai_application_access.verify", "app_id", keyName),
					resource.TestCheckResourceAttr("data.unionai_application_access.verify", "policy_id", policyName),
				),
			},
		},
	})
}

func testAccAppAccessProjectScopedConfig(keyName, projectName, roleName, policyName string) string {
	return fmt.Sprintf(`
provider "unionai" {}

resource "unionai_project" "test" {
  name        = %[2]q
  description = "Acceptance test project"
}

resource "unionai_role" "test" {
  name        = %[3]q
  description = "Acceptance test role"
  actions     = ["view_flyte_executions"]
}

resource "unionai_policy" "test" {
  name = %[4]q

  project {
    id      = unionai_project.test.id
    role_id = unionai_role.test.id
    domains = ["development"]
  }
}

resource "unionai_api_key" "test" {
  id = %[1]q
}

resource "unionai_application_access" "test" {
  app    = unionai_api_key.test.id
  policy = unionai_policy.test.id
}
`, keyName, projectName, roleName, policyName)
}

func testAccAppAccessProjectScopedConfigWithDataSource(keyName, projectName, roleName, policyName string) string {
	return fmt.Sprintf(`
provider "unionai" {}

resource "unionai_project" "test" {
  name        = %[2]q
  description = "Acceptance test project"
}

resource "unionai_role" "test" {
  name        = %[3]q
  description = "Acceptance test role"
  actions     = ["view_flyte_executions"]
}

resource "unionai_policy" "test" {
  name = %[4]q

  project {
    id      = unionai_project.test.id
    role_id = unionai_role.test.id
    domains = ["development"]
  }
}

resource "unionai_api_key" "test" {
  id = %[1]q
}

resource "unionai_application_access" "test" {
  app    = unionai_api_key.test.id
  policy = unionai_policy.test.id
}

data "unionai_application_access" "verify" {
  app_id    = unionai_api_key.test.id
  policy_id = unionai_policy.test.id
}
`, keyName, projectName, roleName, policyName)
}
