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
	_ datasource.DataSource              = (*activeAccountsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*activeAccountsDataSource)(nil)
)

func NewActiveAccountsDataSource() datasource.DataSource {
	return &activeAccountsDataSource{}
}

type activeAccountsDataSource struct {
	client *client.Client
}

type ducklingModel struct {
	ID     types.String `tfsdk:"id"`
	Type   types.String `tfsdk:"type"`
	Status types.String `tfsdk:"status"`
}

type activeAccountModel struct {
	Username  types.String    `tfsdk:"username"`
	Ducklings []ducklingModel `tfsdk:"ducklings"`
}

type activeAccountsModel struct {
	ID       types.String         `tfsdk:"id"`
	Accounts []activeAccountModel `tfsdk:"accounts"`
}

func (d *activeAccountsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_active_accounts"
}

func (d *activeAccountsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Active accounts in the organization and their currently active Ducklings " +
			"(`GET /v1/active_accounts`). Requires the Admin role. This endpoint is in Preview.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Placeholder identifier (always `active_accounts`).",
			},
			"accounts": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Accounts with at least one active Duckling.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"username": schema.StringAttribute{Computed: true},
						"ducklings": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id":     schema.StringAttribute{Computed: true},
									"type":   schema.StringAttribute{Computed: true},
									"status": schema.StringAttribute{Computed: true},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *activeAccountsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *activeAccountsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	accounts, err := d.client.GetActiveAccounts(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list active accounts", err.Error())
		return
	}

	state := activeAccountsModel{
		ID:       types.StringValue("active_accounts"),
		Accounts: make([]activeAccountModel, 0, len(accounts)),
	}
	for _, a := range accounts {
		am := activeAccountModel{
			Username:  types.StringValue(a.Username),
			Ducklings: make([]ducklingModel, 0, len(a.Ducklings)),
		}
		for _, dl := range a.Ducklings {
			am.Ducklings = append(am.Ducklings, ducklingModel{
				ID:     types.StringValue(dl.ID),
				Type:   types.StringValue(dl.Type),
				Status: types.StringValue(dl.Status),
			})
		}
		state.Accounts = append(state.Accounts, am)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
