package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProjectDomainAttributesResource(t *testing.T) {
	suffix := fmt.Sprintf("%06d", rand.Intn(1000000))
	projectName := "tf-acc-project-" + suffix
	roleArn := "arn:aws:iam::000000000000:role/tf-acc-" + suffix

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with a single defaultUserRoleValue attribute.
			{
				Config: testAccProjectDomainAttributesConfig(projectName, roleArn),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_project_domain_attributes.test", "project", projectName),
					resource.TestCheckResourceAttr("unionai_project_domain_attributes.test", "domain", "development"),
					resource.TestCheckResourceAttr("unionai_project_domain_attributes.test", "attributes.defaultUserRoleValue", roleArn),
					resource.TestCheckResourceAttr("unionai_project_domain_attributes.test", "id", projectName+"/development"),
				),
			},
			// Update the attribute value to a different ARN.
			{
				Config: testAccProjectDomainAttributesConfig(projectName, roleArn+"-updated"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_project_domain_attributes.test", "attributes.defaultUserRoleValue", roleArn+"-updated"),
				),
			},
			// Import.
			{
				ResourceName:      "unionai_project_domain_attributes.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccProjectDomainAttributesConfig(projectName, roleArn string) string {
	return fmt.Sprintf(`
provider "unionai" {}

resource "unionai_project" "test" {
  name        = %[1]q
  description = "Acceptance test project"
}

resource "unionai_project_domain_attributes" "test" {
  project = unionai_project.test.id
  domain  = "development"

  attributes = {
    defaultUserRoleValue = %[2]q
  }
}
`, projectName, roleArn)
}
