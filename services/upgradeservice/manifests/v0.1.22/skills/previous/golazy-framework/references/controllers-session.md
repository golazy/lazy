# Controllers/Session

Enable sessions once in `init/app.go`, then use controller helpers. Keep only
browser session identity and flash presentation state here; persistent account
state belongs in a service.

```go
func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{
		Name:         "sample_app",
		Drawer:       Draw,
		Public:       app.Public,
		Views:        app.Views,
		Dependencies: Dependencies,
		Sessions: lazysession.Config{
			Key: os.Getenv("SESSION_KEY"),
		},
	})
}
```

```go
func (c *SessionsController) Create(form *PasswordForm) error {
	user, err := c.accounts.Authenticate(form.Username, form.Password)
	if err != nil {
		return err
	}
	if err := c.SessionSet("user_id", user.ID); err != nil {
		return err
	}
	if err := c.FlashSet("notice", "Signed in"); err != nil {
		return err
	}
	return c.RedirectToRoute("home", lazycontroller.RedirectStatus(http.StatusSeeOther))
}

func (c *SessionsController) Delete() error {
	if err := c.SessionDelete("user_id"); err != nil {
		return err
	}
	return c.RedirectToRoute("home", lazycontroller.RedirectStatus(http.StatusSeeOther))
}
```

Use `SessionGet`, `SessionSet`, `SessionDelete`, `FlashSet`, and `FlashGet` so
GoLazy writes cookies only when needed.

## Related

[Controllers/Base](controllers-base.md) | [Controllers/BeforeFilters](controllers-beforefilters.md)
| [Forms](forms.md) | [Services](services.md)
