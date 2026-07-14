// Package provider implements the MotherDuck Terraform provider over the
// MotherDuck REST API (https://api.motherduck.com/docs/specs).
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

var (
	_ provider.Provider                       = (*motherduckProvider)(nil)
	_ provider.ProviderWithEphemeralResources = (*motherduckProvider)(nil)
)

type motherduckProvider struct {
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &motherduckProvider{version: version}
	}
}

type providerModel struct {
	Token   types.String `tfsdk:"token"`
	BaseURL types.String `tfsdk:"base_url"`
}

func (p *motherduckProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "motherduck"
	resp.Version = p.version
}

func (p *motherduckProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage MotherDuck organization resources (service accounts, access tokens, " +
			"Duckling instance configuration, and Dive embed sessions) via the MotherDuck REST API. " +
			"Authentication requires an access token belonging to a user with the **Admin** role.",
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "MotherDuck access token of an **Admin** user (read/write scope). " +
					"May also be set via the `MOTHERDUCK_API_TOKEN` or `MOTHERDUCK_TOKEN` environment variable.",
			},
			"base_url": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Base URL of the MotherDuck REST API. Defaults to `" + client.DefaultBaseURL + "`. " +
					"May also be set via the `MOTHERDUCK_API_BASE_URL` environment variable.",
			},
		},
	}
}

func (p *motherduckProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := os.Getenv("MOTHERDUCK_API_TOKEN")
	if token == "" {
		token = os.Getenv("MOTHERDUCK_TOKEN")
	}
	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}
	if token == "" {
		resp.Diagnostics.AddError(
			"Missing MotherDuck API token",
			"Set the provider `token` attribute or the MOTHERDUCK_API_TOKEN (or MOTHERDUCK_TOKEN) environment variable. "+
				"The token must belong to a user with the Admin role.",
		)
		return
	}

	baseURL := os.Getenv("MOTHERDUCK_API_BASE_URL")
	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}

	c := client.New(baseURL, token, "terraform-provider-motherduck/"+p.version)
	resp.ResourceData = c
	resp.DataSourceData = c
	resp.EphemeralResourceData = c
}

func (p *motherduckProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewServiceAccountResource,
		NewTokenResource,
		NewDucklingConfigResource,
		NewDatabaseResource,
		NewShareResource,
		NewShareAttachmentResource,
	}
}

func (p *motherduckProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewActiveAccountsDataSource,
		NewTokensDataSource,
		NewDucklingConfigDataSource,
	}
}

func (p *motherduckProvider) EphemeralResources(_ context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewEmbedSessionEphemeralResource,
	}
}
