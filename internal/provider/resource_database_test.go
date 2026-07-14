package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestDatabaseCreateSQL(t *testing.T) {
	got := createDatabaseSQL("example_db")
	want := `CREATE DATABASE "example_db";`
	if got != want {
		t.Fatalf("createDatabaseSQL: got %q, want %q", got, want)
	}
}

func TestDatabaseDropSQL(t *testing.T) {
	got := dropDatabaseSQL("example_db")
	want := `DROP DATABASE "example_db";`
	if got != want {
		t.Fatalf("dropDatabaseSQL: got %q, want %q", got, want)
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
  name  = "tf_provider_acc_demo"
  token = "` + token + `"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_database.demo", "name", "tf_provider_acc_demo"),
					resource.TestCheckResourceAttr("motherduck_database.demo", "id", "tf_provider_acc_demo"),
				),
			},
			{
				ResourceName:            "motherduck_database.demo",
				ImportState:             true,
				ImportStateId:           "tf_provider_acc_demo",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token"},
			},
		},
	})
}
