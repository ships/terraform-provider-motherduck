package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

var (
	_ datasource.DataSource              = (*tokensDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*tokensDataSource)(nil)
)

func NewTokensDataSource() datasource.DataSource {
	return &tokensDataSource{}
}

type tokensDataSource struct {
	client *client.Client
}

type tokenListItemModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	TokenType types.String `tfsdk:"token_type"`
	CreatedAt types.String `tfsdk:"created_at"`
	ExpiresAt types.String `tfsdk:"expires_at"`
	ReadOnly  types.Bool   `tfsdk:"read_only"`
}

type tokensModel struct {
	ID       types.String         `tfsdk:"id"`
	Username types.String         `tfsdk:"username"`
	Tokens   []tokenListItemModel `tfsdk:"tokens"`
}

func (d *tokensDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tokens"
}

func (d *tokensDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Access tokens of a MotherDuck user (`GET /v1/users/{username}/tokens`). " +
			"Secret token values are never returned by this endpoint.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `username`.",
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "User whose tokens to list.",
			},
			"tokens": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Token ID (UUID)."},
						"name":       schema.StringAttribute{Computed: true},
						"token_type": schema.StringAttribute{Computed: true, MarkdownDescription: "`read_write` or `read_scaling`."},
						"created_at": schema.StringAttribute{Computed: true},
						"expires_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Empty for non-expiring tokens."},
						"read_only":  schema.BoolAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *tokensDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *tokensDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config tokensModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tokens, err := d.client.ListTokens(ctx, config.Username.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to list tokens", err.Error())
		return
	}

	config.ID = config.Username
	config.Tokens = make([]tokenListItemModel, 0, len(tokens))
	for _, t := range tokens {
		config.Tokens = append(config.Tokens, tokenListItemModel{
			ID:        types.StringValue(t.ID),
			Name:      types.StringValue(t.Name),
			TokenType: types.StringValue(string(t.TokenType)),
			CreatedAt: types.StringValue(t.CreatedTS),
			ExpiresAt: types.StringValue(t.ExpireAt),
			ReadOnly:  types.BoolValue(t.ReadOnly),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
