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
			{
				Config: testAccAppAccessConfig(keyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_application_access.test", "app", keyName),
					resource.TestCheckResourceAttr("unionai_application_access.test", "policy", "contributor"),
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
