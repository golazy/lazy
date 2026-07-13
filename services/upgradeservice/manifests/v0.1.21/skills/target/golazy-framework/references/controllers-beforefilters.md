# Controllers/BeforeFilters

Use `BeforeAction` for shared request presentation setup and authorization.
Receive generated values instead of repeating session or request parsing.

```go
// app/controllers/base_controller.go
type User struct {
	ID string
}

type AuthenticatedUser struct {
	*User
}

func (c *BaseController) GenUser() (*User, error) {
	value, ok, err := c.SessionGet("user_id")
	if err != nil {
		return nil, err
	}
	userID, _ := value.(string)
	if !ok || userID == "" {
		return nil, nil
	}
	return &User{ID: userID}, nil
}

func (c *BaseController) GenAuthenticatedUser(user *User) (*AuthenticatedUser, error) {
	if user == nil {
		return nil, ErrLoginRequired
	}
	return &AuthenticatedUser{User: user}, nil
}

func (c *BaseController) BeforeAction(user *User) error {
	c.Set("current_user", user)
	return nil
}
```

```go
func (c *AdminController) Index(user *controllers.AuthenticatedUser) error {
	c.Set("user", user)
	return nil
}
```

Use a wrapper type such as `AuthenticatedUser` to make authorization visible
in an action signature. Keep business permissions in a service when they are
more than request-bound access control.

## Related

[Controllers/Base](controllers-base.md) | [Controllers/Generators](controllers-generators.md)
| [Controllers/Session](controllers-session.md)
