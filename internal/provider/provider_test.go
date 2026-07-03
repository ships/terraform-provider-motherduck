package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"motherduck": providerserver.NewProtocol6WithError(New("test")()),
}

func providerConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "motherduck" {
  token    = "test-token"
  base_url = %q
}
`, baseURL)
}

func TestAccServiceAccount(t *testing.T) {
	api := newMockAPI()
	baseURL := api.start(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(baseURL) + `
resource "motherduck_service_account" "etl" {
  username = "svc_etl"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_service_account.etl", "username", "svc_etl"),
					resource.TestCheckResourceAttr("motherduck_service_account.etl", "id", "svc_etl"),
				),
			},
			{
				ResourceName:      "motherduck_service_account.etl",
				ImportState:       true,
				ImportStateId:     "svc_etl",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccServiceAccountInvalidUsername(t *testing.T) {
	api := newMockAPI()
	baseURL := api.start(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(baseURL) + `
resource "motherduck_service_account" "bad" {
  username = "has-a-hyphen"
}
`,
				// Caught at plan time by the validator, before any API call.
				ExpectError: regexp.MustCompile(`letters, numbers, and underscores`),
			},
		},
	})
}

func TestAccToken(t *testing.T) {
	api := newMockAPI()
	baseURL := api.start(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(baseURL) + `
resource "motherduck_service_account" "etl" {
  username = "svc_etl"
}

resource "motherduck_token" "ci" {
  username    = motherduck_service_account.etl.username
  name        = "ci-token"
  ttl_seconds = 3600
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_token.ci", "name", "ci-token"),
					resource.TestCheckResourceAttr("motherduck_token.ci", "token_type", "read_write"),
					resource.TestCheckResourceAttrSet("motherduck_token.ci", "token"),
					resource.TestCheckResourceAttrSet("motherduck_token.ci", "id"),
				),
			},
			{
				// Changing the name forces a replacement (no update API).
				Config: providerConfig(baseURL) + `
resource "motherduck_service_account" "etl" {
  username = "svc_etl"
}

resource "motherduck_token" "ci" {
  username    = motherduck_service_account.etl.username
  name        = "ci-token-v2"
  ttl_seconds = 3600
  token_type  = "read_scaling"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_token.ci", "name", "ci-token-v2"),
					resource.TestCheckResourceAttr("motherduck_token.ci", "token_type", "read_scaling"),
					resource.TestCheckResourceAttr("motherduck_token.ci", "read_only", "true"),
				),
			},
		},
	})
}

func TestAccTokenInvalidTTL(t *testing.T) {
	api := newMockAPI()
	baseURL := api.start(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(baseURL) + `
resource "motherduck_token" "bad" {
  username    = "whoever"
  name        = "too-short"
  ttl_seconds = 10
}
`,
				ExpectError: regexp.MustCompile(`ttl_seconds`),
			},
		},
	})
}

func TestAccDucklingConfig(t *testing.T) {
	api := newMockAPI()
	baseURL := api.start(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(baseURL) + `
resource "motherduck_service_account" "etl" {
  username = "svc_etl"
}

resource "motherduck_duckling_config" "etl" {
  username = motherduck_service_account.etl.username

  read_write {
    instance_size    = "jumbo"
    cooldown_seconds = 300
  }

  read_scaling {
    instance_size = "standard"
    flock_size    = 2
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_duckling_config.etl", "read_write.instance_size", "jumbo"),
					resource.TestCheckResourceAttr("motherduck_duckling_config.etl", "read_write.cooldown_seconds", "300"),
					resource.TestCheckResourceAttr("motherduck_duckling_config.etl", "read_scaling.flock_size", "2"),
				),
			},
			{
				// In-place update (PUT), no replacement.
				Config: providerConfig(baseURL) + `
resource "motherduck_service_account" "etl" {
  username = "svc_etl"
}

resource "motherduck_duckling_config" "etl" {
  username = motherduck_service_account.etl.username

  read_write {
    instance_size    = "mega"
    cooldown_seconds = 300
  }

  read_scaling {
    instance_size = "standard"
    flock_size    = 4
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_duckling_config.etl", "read_write.instance_size", "mega"),
					resource.TestCheckResourceAttr("motherduck_duckling_config.etl", "read_scaling.flock_size", "4"),
				),
			},
		},
	})
}

func TestAccDataSources(t *testing.T) {
	api := newMockAPI()
	baseURL := api.start(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(baseURL) + `
resource "motherduck_service_account" "etl" {
  username = "svc_etl"
}

resource "motherduck_token" "ci" {
  username = motherduck_service_account.etl.username
  name     = "ci-token"
}

data "motherduck_tokens" "etl" {
  username   = motherduck_service_account.etl.username
  depends_on = [motherduck_token.ci]
}

data "motherduck_active_accounts" "all" {
  depends_on = [motherduck_service_account.etl]
}

data "motherduck_duckling_config" "etl" {
  username   = motherduck_service_account.etl.username
  depends_on = [motherduck_service_account.etl]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.motherduck_tokens.etl", "tokens.#", "1"),
					resource.TestCheckResourceAttr("data.motherduck_tokens.etl", "tokens.0.name", "ci-token"),
					resource.TestCheckResourceAttr("data.motherduck_active_accounts.all", "accounts.#", "1"),
					resource.TestCheckResourceAttr("data.motherduck_active_accounts.all", "accounts.0.username", "svc_etl"),
					resource.TestCheckResourceAttr("data.motherduck_duckling_config.etl", "read_write.instance_size", "pulse"),
				),
			},
		},
	})
}
