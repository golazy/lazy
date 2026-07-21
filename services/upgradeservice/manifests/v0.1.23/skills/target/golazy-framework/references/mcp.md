# MCP

MCP modules under `app/mcps` are communication adapters. Tools validate and
translate MCP input, call services, and return typed MCP results. They do not
own business rules.

```go
// app/mcps/admin_mcp/adminmcp.go
type AdminMCP struct {
	lazymcp.Base
	accounts accountservice.Service
}

func New(ctx context.Context) (*AdminMCP, error) {
	accounts, ok := accountservice.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("account service is missing from MCP context")
	}
	return &AdminMCP{
		Base:     lazymcp.NewBase(ctx),
		accounts: accounts,
	}, nil
}

type UserCountParams struct {
	Active bool `json:"active"`
}

type UserCountResult struct {
	Count int `json:"count"`
}

func (m *AdminMCP) UserCountTool(ctx context.Context) lazymcp.ToolSpec {
	return lazymcp.ToolSpec{
		Desc: "Count users.",
		Fn:   m.UserCount,
		UI:   lazymcp.UI("ui://admin/dashboard"),
	}
}

func (m *AdminMCP) UserCount(ctx context.Context, params UserCountParams) (UserCountResult, error) {
	count, err := m.accounts.Count(ctx, accountservice.CountFilter{Active: params.Active})
	return UserCountResult{Count: count}, err
}

func (m *AdminMCP) DashboardApp(ctx context.Context) lazymcp.AppSpec {
	return lazymcp.AppSpec{
		Name:          "dashboard",
		Desc:          "Admin dashboard.",
		View:          "dashboard",
		UseLayout:     true,
		Domain:        "https://mcp.example.com",
		CSP: lazymcp.AppCSP{
			ConnectDomains:  []string{},
			ResourceDomains: []string{},
		},
	}
}
```

GoLazy reflects the first tool return value into `outputSchema`. Keep result
structs and JSON tags accurate; set `ToolSpec.OutputSchema` only for a dynamic
or custom-encoded result shape. For Apps, set `Domain` to the dedicated HTTPS
widget origin and declare only required outbound origins in `CSP`. Empty slices
are a valid policy for a self-contained widget.

Keep large or widget-only values out of `structuredContent`, because the model
reads it verbatim. A result can implement `lazymcp.ResultMetadata` by defining
`MCPResultMeta() map[string]any`; GoLazy then emits that map as the tool
result's top-level `_meta`. Mark those fields `json:"-"` on the typed result so
the reflected output schema, text fallback, and structured content stay small.
MCP Apps receive `_meta` through `ui/notifications/tool-result`; ChatGPT also
preserves it inside `window.openai.toolResponseMetadata`. Treat `_meta` as
user-visible widget data, never as storage for credentials or server secrets.

```go
// init/mcp.go
func RegisterMCP(ctx context.Context, scope *lazymcp.Scope) error {
	admin, err := adminmcp.New(ctx)
	if err != nil {
		return err
	}
	return scope.Register(admin)
}
```

```go
// init/app.go
func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{
		Name:         "sample_app",
		Drawer:       Draw,
		Public:       app.Public,
		Views:        app.Views,
		Dependencies: Dependencies,
		MCP:          RegisterMCP,
	})
}
```

```gotemplate
{{/* app/views/mcp/admin/dashboard.html.tpl */}}
<section>
  <h1>Admin</h1>
  <p>Rendered as an MCP Apps UI resource.</p>
</section>
```

Apply authorization at the adapter boundary and repeat critical permission
checks inside the service when they are domain policy. Keep tool descriptions
specific enough for a model to choose correctly.

## OAuth Login Redirects

The built-in `lazyoauth` login page is standalone HTML. It does not render the
application layout or load Turbo, Stimulus, or app JavaScript. Its form posts
explicitly to the server's authorization endpoint.

Do not add `form-action 'self'` to an OAuth login page that completes with a
cross-origin callback redirect. Chromium may carry that CSP restriction across
the successful form POST and block navigation to the client's validated
redirect URI. Keep the credential POST same-origin through the explicit form
action, and let `lazyoauth` enforce exact redirect-URI matching server-side.
Cover the flow with a POST test that expects `302 Found` and the external
client callback in `Location`.

## Related

[Services](services.md) | [Views](views.md) | [Controllers/Base](controllers-base.md)
| [Testing](testing-verification.md)
