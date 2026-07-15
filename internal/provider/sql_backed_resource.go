package provider

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

// sqlBackedResource is composed by every resource whose CRUD is SQL DDL rather
// than a REST call. The account the DDL runs as is a per-resource input token,
// not provider config — so one provider can manage objects for many accounts,
// and a resource may reference a motherduck_token created in the same apply.
type sqlBackedResource struct{}

// tokenAttribute is the per-resource data-plane token. LengthAtLeast(1) rejects
// the empty string at plan time: an empty token would otherwise resolve to a
// local in-memory duckdb (see client.bootstrap) and silently target a throwaway
// database instead of MotherDuck, masking misconfiguration as a no-op.
func (sqlBackedResource) tokenAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required:    true,
		Sensitive:   true,
		Description: "Data-plane token of the account this object is owned/created by (e.g. motherduck_token.x.token).",
		Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
	}
}

func (sqlBackedResource) clientFor(token string) *client.SQLClient {
	return client.NewSQLClient(token)
}

// splitImportID parses a `<token>,<name>` import identifier. The token carries
// no comma (MotherDuck tokens are dot-delimited base64url), so the first comma
// is the separator and the remainder is the object name/alias verbatim. Import
// must carry the token because it is not recoverable from the managed object and
// Read needs it to reach the owning account.
func splitImportID(id string) (token, name string, ok bool) {
	token, name, ok = strings.Cut(id, ",")
	if !ok || token == "" || name == "" {
		return "", "", false
	}
	return token, name, true
}
