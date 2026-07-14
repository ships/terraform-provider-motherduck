package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAttachShareSQL(t *testing.T) {
	got := attachShareSQL("md:_share/ducks/0a9a026ec5a55946a9de39851087ed81", "birds")
	want := `ATTACH 'md:_share/ducks/0a9a026ec5a55946a9de39851087ed81' AS "birds";`
	if got != want {
		t.Fatalf("attachShareSQL: got %q, want %q", got, want)
	}
}

func TestAttachShareSQLQuotesURLAndAlias(t *testing.T) {
	got := attachShareSQL("md:_share/o'brien/tok", `weird"alias`)
	want := `ATTACH 'md:_share/o''brien/tok' AS "weird""alias";`
	if got != want {
		t.Fatalf("attachShareSQL: got %q, want %q", got, want)
	}
}

func TestDetachSQL(t *testing.T) {
	got := detachSQL("birds")
	want := `DETACH "birds";`
	if got != want {
		t.Fatalf("detachSQL: got %q, want %q", got, want)
	}
}

func TestDetachSQLQuotesAlias(t *testing.T) {
	got := detachSQL(`weird"alias`)
	want := `DETACH "weird""alias";`
	if got != want {
		t.Fatalf("detachSQL: got %q, want %q", got, want)
	}
}

func TestListAttachedSharesSQL(t *testing.T) {
	got := listAttachedSharesSQL()
	want := `SELECT alias, is_attached, type, fully_qualified_name ` +
		`FROM MD_ALL_DATABASES() WHERE type = 'MotherDuck share' AND is_attached = true;`
	if got != want {
		t.Fatalf("listAttachedSharesSQL: got %q, want %q", got, want)
	}
}

func TestAccShareAttachment_basic(t *testing.T) {
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
  name  = "tf_provider_acc_attach_src"
  token = "` + token + `"
}

resource "motherduck_share" "demo" {
  name     = "tf_provider_acc_attach_share"
  database = motherduck_database.src.name
  token    = "` + token + `"
  access   = "unrestricted"
}

resource "motherduck_share_attachment" "demo" {
  share_url = motherduck_share.demo.share_url
  alias     = "tf_provider_acc_attach_alias"
  token     = "` + token + `"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("motherduck_share_attachment.demo", "alias", "tf_provider_acc_attach_alias"),
					resource.TestCheckResourceAttr("motherduck_share_attachment.demo", "id", "tf_provider_acc_attach_alias"),
				),
			},
			{
				ResourceName:            "motherduck_share_attachment.demo",
				ImportState:             true,
				ImportStateId:           "tf_provider_acc_attach_alias",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token", "share_url"},
			},
		},
	})
}
