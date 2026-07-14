package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestShareCreateSQLRestricted(t *testing.T) {
	got, err := createShareSQL("analytics_share", "analytics", "restricted")
	if err != nil {
		t.Fatalf("createShareSQL: unexpected error %v", err)
	}
	want := `CREATE SHARE "analytics_share" FROM "analytics" (ACCESS RESTRICTED);`
	if got != want {
		t.Fatalf("createShareSQL: got %q, want %q", got, want)
	}
}

func TestShareCreateSQLUnrestricted(t *testing.T) {
	got, err := createShareSQL("public_share", "analytics", "unrestricted")
	if err != nil {
		t.Fatalf("createShareSQL: unexpected error %v", err)
	}
	want := `CREATE SHARE "public_share" FROM "analytics" (ACCESS UNRESTRICTED);`
	if got != want {
		t.Fatalf("createShareSQL: got %q, want %q", got, want)
	}
}

func TestShareCreateSQLRejectsUnknownAccess(t *testing.T) {
	if _, err := createShareSQL("s", "db", "organization"); err == nil {
		t.Fatalf("createShareSQL: expected error for unknown access, got nil")
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
  name  = "tf_provider_acc_share_src"
  token = "` + token + `"
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
					resource.TestCheckResourceAttrSet("motherduck_share.demo", "share_url"),
				),
			},
			{
				ResourceName:            "motherduck_share.demo",
				ImportState:             true,
				ImportStateId:           "tf_provider_acc_share",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token", "grant_to", "database", "access"},
			},
		},
	})
}
