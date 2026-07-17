package provider

import (
	"context"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

func TestShareCreateSQLAutomatic(t *testing.T) {
	cases := []struct {
		name, access, want string
	}{
		{"restricted", "restricted", `CREATE SHARE "s" FROM "db" (ACCESS RESTRICTED, UPDATE AUTOMATIC);`},
		{"unrestricted", "unrestricted", `CREATE SHARE "s" FROM "db" (ACCESS UNRESTRICTED, UPDATE AUTOMATIC);`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := createShareSQL("s", "db", c.access, "automatic")
			if err != nil {
				t.Fatalf("createShareSQL: unexpected error %v", err)
			}
			if got != c.want {
				t.Fatalf("createShareSQL: got %q, want %q", got, c.want)
			}
		})
	}
}

func TestShareCreateSQLManual(t *testing.T) {
	got, err := createShareSQL("s", "db", "restricted", "manual")
	if err != nil {
		t.Fatalf("createShareSQL: unexpected error %v", err)
	}
	want := `CREATE SHARE "s" FROM "db" (ACCESS RESTRICTED, UPDATE MANUAL);`
	if got != want {
		t.Fatalf("createShareSQL: got %q, want %q", got, want)
	}
}

func TestShareCreateSQLRejectsUnknownAccess(t *testing.T) {
	if _, err := createShareSQL("s", "db", "organization", "automatic"); err == nil {
		t.Fatalf("createShareSQL: expected error for unknown access, got nil")
	}
}

func TestShareCreateSQLRejectsUnknownUpdateMode(t *testing.T) {
	if _, err := createShareSQL("s", "db", "restricted", "snapshot"); err == nil {
		t.Fatalf("createShareSQL: expected error for unknown update mode, got nil")
	}
}

func TestIsDuckLakeType(t *testing.T) {
	cases := map[string]bool{
		"motherduck":          false,
		"motherduck ducklake": true,
		"motherduck share":    false,
		"ducklake":            true,
	}
	for dbType, want := range cases {
		if got := isDuckLakeType(dbType); got != want {
			t.Fatalf("isDuckLakeType(%q): got %v, want %v", dbType, got, want)
		}
	}
}

func TestShareDropSQL(t *testing.T) {
	got := dropShareSQL("analytics_share")
	want := `DROP SHARE "analytics_share";`
	if got != want {
		t.Fatalf("dropShareSQL: got %q, want %q", got, want)
	}
}

func TestShareURLSQL(t *testing.T) {
	got := shareUrlSQL("analytics_share")
	want := `SELECT name, url, access, visibility, source_db_name ` +
		`FROM MD_INFORMATION_SCHEMA.OWNED_SHARES WHERE name = 'analytics_share';`
	if got != want {
		t.Fatalf("shareUrlSQL: got %q, want %q", got, want)
	}
}

func TestShareGrantReadSQL(t *testing.T) {
	got := grantReadSQL("analytics_share", []string{"user_1", "user_2@example-com"})
	want := `GRANT READ ON SHARE "analytics_share" TO "user_1", "user_2@example-com";`
	if got != want {
		t.Fatalf("grantReadSQL: got %q, want %q", got, want)
	}
}

func TestShareRevokeReadSQL(t *testing.T) {
	got := revokeReadSQL("analytics_share", "penguin")
	want := `REVOKE READ ON SHARE "analytics_share" FROM "penguin";`
	if got != want {
		t.Fatalf("revokeReadSQL: got %q, want %q", got, want)
	}
}

func TestAccShare_basic(t *testing.T) {
	if os.Getenv("MOTHERDUCK_TEST_TOKEN") == "" {
		t.Skip("requires live MotherDuck")
	}
	token := os.Getenv("MOTHERDUCK_TEST_TOKEN")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "motherduck_database" "src" {
  name            = "tf_provider_acc_share_src"
  token           = "` + token + `"
  deletion_policy = "cascade"
}

resource "motherduck_share" "demo" {
  name     = "tf_provider_acc_share"
  database = motherduck_database.src.name
  token    = "` + token + `"
  access   = "restricted"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_share.demo", "name", "tf_provider_acc_share"),
					resource.TestCheckResourceAttr("motherduck_share.demo", "access", "restricted"),
					resource.TestCheckResourceAttr("motherduck_share.demo", "update_mode", "automatic"),
					resource.TestCheckResourceAttrSet("motherduck_share.demo", "share_url"),
				),
			},
			{
				ResourceName:                         "motherduck_share.demo",
				ImportState:                          true,
				ImportStateId:                        token + ",tf_provider_acc_share",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"grant_to", "database", "access", "update_mode"},
			},
		},
	})
}

// TestAccShareDuckLake covers the update-mode grid over a DuckLake source, whose
// only legal mode is automatic. Both the default (update_mode omitted) and the
// explicit-automatic config must succeed; a manual request must be rejected by the
// provider before any DDL runs. The DuckLake is created out-of-band (the database
// resource has no TYPE DUCKLAKE surface) and torn down via t.Cleanup.
func TestAccShareDuckLake(t *testing.T) {
	if os.Getenv("MOTHERDUCK_TEST_TOKEN") == "" {
		t.Skip("requires live MotherDuck")
	}
	token := os.Getenv("MOTHERDUCK_TEST_TOKEN")

	const dbName = "tf_provider_acc_dl_src"
	const shareName = "tf_provider_acc_dl_share"
	sql := client.NewSQLClient(token)
	ctx := context.Background()

	drop := func() {
		_ = sql.Exec(ctx, `DROP SHARE IF EXISTS "`+shareName+`";`)
		_ = sql.Exec(ctx, `DROP DATABASE IF EXISTS "`+dbName+`";`)
	}
	drop()
	if err := sql.Exec(ctx, `CREATE DATABASE "`+dbName+`" (TYPE DUCKLAKE);`); err != nil {
		t.Fatalf("create ducklake source: %v", err)
	}
	t.Cleanup(drop)

	shareConfig := func(extra string) string {
		return `
resource "motherduck_share" "lake" {
  name     = "` + shareName + `"
  database = "` + dbName + `"
  token    = "` + token + `"
  access   = "restricted"` + extra + `
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: shareConfig(""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_share.lake", "name", shareName),
					resource.TestCheckResourceAttr("motherduck_share.lake", "update_mode", "automatic"),
					resource.TestCheckResourceAttrSet("motherduck_share.lake", "share_url"),
				),
			},
			{
				Config: shareConfig("\n  update_mode = \"automatic\""),
				Check: resource.TestCheckResourceAttr(
					"motherduck_share.lake", "update_mode", "automatic"),
			},
			{
				Config:      shareConfig("\n  update_mode = \"manual\""),
				ExpectError: regexp.MustCompile(`(?s)only be shared in automatic update mode`),
			},
		},
	})
}

// TestAccShareDuckDBManual covers manual update mode, legal only for a standard
// database. The source is Terraform-managed; the share pins update_mode = manual.
func TestAccShareDuckDBManual(t *testing.T) {
	if os.Getenv("MOTHERDUCK_TEST_TOKEN") == "" {
		t.Skip("requires live MotherDuck")
	}
	token := os.Getenv("MOTHERDUCK_TEST_TOKEN")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "motherduck_database" "src" {
  name            = "tf_provider_acc_manual_src"
  token           = "` + token + `"
  deletion_policy = "cascade"
}

resource "motherduck_share" "manual" {
  name        = "tf_provider_acc_manual_share"
  database    = motherduck_database.src.name
  token       = "` + token + `"
  access      = "restricted"
  update_mode = "manual"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_share.manual", "update_mode", "manual"),
					resource.TestCheckResourceAttrSet("motherduck_share.manual", "share_url"),
				),
			},
		},
	})
}
