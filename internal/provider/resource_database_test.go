package provider

import (
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestDatabaseCreateSQL(t *testing.T) {
	got := createDatabaseSQL("example_db")
	want := `CREATE DATABASE "example_db";`
	if got != want {
		t.Fatalf("createDatabaseSQL: got %q, want %q", got, want)
	}
}

func TestDatabaseDropCascadeSQL(t *testing.T) {
	got := dropDatabaseCascadeSQL("example_db")
	want := `DROP DATABASE "example_db" CASCADE;`
	if got != want {
		t.Fatalf("dropDatabaseCascadeSQL: got %q, want %q", got, want)
	}
}

func TestDatabaseListSQL(t *testing.T) {
	got := listDatabasesSQL()
	want := `SHOW ALL DATABASES;`
	if got != want {
		t.Fatalf("listDatabasesSQL: got %q, want %q", got, want)
	}
}

func TestDatabaseQuoteIdentEscapesEmbeddedQuote(t *testing.T) {
	got := quoteIdent(`ab"c`)
	want := `"ab""c"`
	if got != want {
		t.Fatalf("quoteIdent: got %q, want %q", got, want)
	}
}

func TestAccDatabase_basic(t *testing.T) {
	if os.Getenv("MOTHERDUCK_TEST_TOKEN") == "" {
		t.Skip("requires live MotherDuck")
	}
	token := os.Getenv("MOTHERDUCK_TEST_TOKEN")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "motherduck_database" "demo" {
  name            = "tf_provider_acc_demo"
  token           = "` + token + `"
  deletion_policy = "cascade"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_database.demo", "name", "tf_provider_acc_demo"),
					resource.TestCheckResourceAttr("motherduck_database.demo", "id", "tf_provider_acc_demo"),
				),
			},
			{
				ResourceName:      "motherduck_database.demo",
				ImportState:       true,
				ImportStateId:     token + ",tf_provider_acc_demo",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccDatabase_deletionPolicyPrevent(t *testing.T) {
	if os.Getenv("MOTHERDUCK_TEST_TOKEN") == "" {
		t.Skip("requires live MotherDuck")
	}
	token := os.Getenv("MOTHERDUCK_TEST_TOKEN")

	prevent := `
resource "motherduck_database" "guard" {
  name            = "tf_provider_acc_guard"
  token           = "` + token + `"
  deletion_policy = "prevent"
}
`
	cascade := `
resource "motherduck_database" "guard" {
  name            = "tf_provider_acc_guard"
  token           = "` + token + `"
  deletion_policy = "cascade"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: prevent,
				Check:  resource.TestCheckResourceAttr("motherduck_database.guard", "deletion_policy", "prevent"),
			},
			{
				Config:      prevent,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`deletion_policy`),
			},
			{
				Config: cascade,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("motherduck_database.guard", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}
