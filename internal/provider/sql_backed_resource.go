package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

// sqlBackedResource is composed by every resource whose CRUD is SQL DDL rather
// than a REST call. The account the DDL runs as is a per-resource input token,
// not provider config — so one provider can manage objects for many accounts,
// and a resource may reference a motherduck_token created in the same apply.
type sqlBackedResource struct{}

func (sqlBackedResource) tokenAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required:    true,
		Sensitive:   true,
		Description: "Data-plane token of the account this object is owned/created by (e.g. motherduck_token.x.token).",
	}
}

func (sqlBackedResource) clientFor(token string) *client.SQLClient {
	return client.NewSQLClient(token)
}
