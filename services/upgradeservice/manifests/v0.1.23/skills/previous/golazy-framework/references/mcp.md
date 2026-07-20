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
		Name:      "dashboard",
		Desc:      "Admin dashboard.",
		View:      "dashboard",
		UseLayout: true,
	}
}
```

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

## Related

[Services](services.md) | [Views](views.md) | [Controllers/Base](controllers-base.md)
| [Testing](testing-verification.md)
