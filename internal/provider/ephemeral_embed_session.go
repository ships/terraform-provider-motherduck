package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

var (
	_ ephemeral.EphemeralResource              = (*embedSessionEphemeralResource)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*embedSessionEphemeralResource)(nil)
)

func NewEmbedSessionEphemeralResource() ephemeral.EphemeralResource {
	return &embedSessionEphemeralResource{}
}

type embedSessionEphemeralResource struct {
	client *client.Client
}

type embedResourceModel struct {
	URL   types.String `tfsdk:"url"`
	Alias types.String `tfsdk:"alias"`
}

type embedSessionModel struct {
	DiveID            types.String         `tfsdk:"dive_id"`
	Username          types.String         `tfsdk:"username"`
	SessionHint       types.String         `tfsdk:"session_hint"`
	RequiredResources []embedResourceModel `tfsdk:"required_resources"`
	InitialState      types.String         `tfsdk:"initial_state"`
	Version           types.Int64          `tfsdk:"version"`
	Session           types.String         `tfsdk:"session"`
}

func (r *embedSessionEphemeralResource) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_embed_session"
}

func (r *embedSessionEphemeralResource) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An embed session for a MotherDuck Dive, created on behalf of a service account " +
			"(`POST /v1/dives/{dive_id}/embed-session`). Sessions are short-lived credentials, so this is an " +
			"ephemeral resource (requires Terraform 1.10+): the session token is never persisted to state or plan.",
		Attributes: map[string]schema.Attribute{
			"dive_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the Dive to embed.",
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Service account the session is created for.",
			},
			"session_hint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Opaque hint used to partition sessions (for example, an end-user or tenant ID).",
			},
			"required_resources": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Database resources the session needs access to.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"url": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Resource URL (for example a share URL).",
						},
						"alias": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Alias to attach the resource under.",
						},
					},
				},
			},
			"initial_state": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "JSON-encoded initial state for the embedded Dive.",
			},
			"version": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Embed session protocol version.",
			},
			"session": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The embed session token.",
			},
		},
	}
}

func (r *embedSessionEphemeralResource) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *embedSessionEphemeralResource) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var config embedSessionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateEmbedSessionRequest{
		Username:    config.Username.ValueString(),
		SessionHint: config.SessionHint.ValueString(),
	}
	for _, res := range config.RequiredResources {
		createReq.RequiredResources = append(createReq.RequiredResources, client.EmbedResource{
			URL:   res.URL.ValueString(),
			Alias: res.Alias.ValueString(),
		})
	}
	if !config.InitialState.IsNull() {
		raw := json.RawMessage(config.InitialState.ValueString())
		if !json.Valid(raw) {
			resp.Diagnostics.AddError("Invalid initial_state", "initial_state must be a valid JSON document")
			return
		}
		createReq.InitialState = raw
	}
	if !config.Version.IsNull() {
		v := config.Version.ValueInt64()
		createReq.Version = &v
	}

	session, err := r.client.CreateEmbedSession(ctx, config.DiveID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create embed session", err.Error())
		return
	}

	config.Session = types.StringValue(session)
	resp.Diagnostics.Append(resp.Result.Set(ctx, &config)...)
}
