package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAppAccessResource_Basic(t *testing.T) {
	keyName := "tf-acc-test-app-access"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the API key and assign it to the policy
			{
				Config: testAccAppAccessConfig(keyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_application_access.test", "app", keyName),
					resource.TestCheckResourceAttr("unionai_application_access.test", "policy", "contributor"),
				),
			},
			// Step 2: Verify the assignment exists server-side via the data source
			{
				Config: testAccAppAccessConfigWithDataSource(keyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.unionai_application_access.verify", "app_id", keyName),
					resource.TestCheckResourceAttr("data.unionai_application_access.verify", "policy_id", "contributor"),
				),
			},
		},
	})
}

func testAccAppAccessConfig(keyName string) string {
	return fmt.Sprintf(`
provider "unionai" {}

resource "unionai_api_key" "test" {
  id = %[1]q
}

data "unionai_policy" "contributor" {
  id = "contributor"
}

resource "unionai_application_access" "test" {
  app    = unionai_api_key.test.id
  policy = data.unionai_policy.contributor.id
}
`, keyName)
}

func testAccAppAccessConfigWithDataSource(keyName string) string {
	return fmt.Sprintf(`
provider "unionai" {}

resource "unionai_api_key" "test" {
  id = %[1]q
}

data "unionai_policy" "contributor" {
  id = "contributor"
}

resource "unionai_application_access" "test" {
  app    = unionai_api_key.test.id
  policy = data.unionai_policy.contributor.id
}

data "unionai_application_access" "verify" {
  app_id    = unionai_api_key.test.id
  policy_id = data.unionai_policy.contributor.id
}
`, keyName)
}
