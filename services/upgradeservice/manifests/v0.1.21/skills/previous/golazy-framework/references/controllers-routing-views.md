# Controllers, Routing, And Views

Controllers are request-local. Routes point at controller constructors, not
shared controller instances.

## Controller Shape

```go
type PostsController struct {
	controllers.BaseController
	posts postservice.Service
}

func New(ctx context.Context) (*PostsController, error) {
	base, err := controllers.NewBaseController(ctx)
	if err != nil {
		return nil, err
	}
	posts, ok := postservice.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("posts service missing")
	}
	return &PostsController{BaseController: base, posts: posts}, nil
}
```

Constructor rules:

- Accept only `context.Context`.
- Return `(*Controller, error)`.
- Build the shared base controller first.
- Resolve services through typed context helpers.
- Fail early when a required service is missing.

## Action Shape

The standard action signature uses `net/http` values and returns `error`:

```go
func (c *PostsController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.Set("title", "Posts")
	c.Set("posts", c.posts.List())
	return nil
}
```

If the action returns `nil` without writing a response, GoLazy renders the
default view for the matched controller/action. Use `c.Render("other")` only
when an action intentionally renders a different view.

Use `c.Status`, `c.Header`, and `c.ContentType` to set metadata before the
automatic render. Use the raw `http.ResponseWriter` only when the action owns
the whole response body.

## Generated Action And Hook Arguments

Use generator arguments when an action or `BeforeAction` hook should receive a
typed request-derived value instead of reading untyped state from the
controller:

```go
type User struct {
	ID string
}

type AuthenticatedUser struct {
	*User
}

var errLoginRequired = errors.New("login required")

func (c *BaseController) GenUser() (*User, error) {
	value, ok, err := c.SessionGet("user_id")
	if err != nil {
		return nil, err
	}
	userID, _ := value.(string)
	if !ok || userID == "" {
		return nil, nil
	}
	return c.users.FindByID(userID)
}

func (c *BaseController) GenAuthenticatedUser(
	user *User,
) (*AuthenticatedUser, error) {
	if user == nil {
		return nil, errLoginRequired
	}
	return &AuthenticatedUser{User: user}, nil
}

func (c *BaseController) BeforeAction(user *User) error {
	c.Set("current_user", user)
	return nil
}

func (c *AdminController) Index(user *AuthenticatedUser) error {
	return nil
}
```

`GenX` methods may receive `context.Context`, `*http.Request`, route
parameters, and other generated values. They return `T` or `(T, error)`.
Generated values are cached by type inside the current controller request, so
`BeforeAction` and the routed action share the same generated value.

When a generator returns a non-nil error, GoLazy does not call the action or
hook that requested it. It passes the error through the normal controller error
path, including `HandleError(http.ResponseWriter, *http.Request, error)` when
the concrete controller or embedded base controller implements it. For
protected areas, keep auth wrapper types such as `AuthenticatedUser` in the
application, generate them from the optional current user, and map
`errLoginRequired` to a redirect or status in the application base controller's
`HandleError`.

## Typed Form Generators

For submitted forms, put the input struct in its own file near the controller
and generate it for the action:

```go
// password_form.go
type PasswordForm struct {
	Username string
	Password string
}

func (c *SessionsController) GenPasswordForm() (*PasswordForm, error) {
	form := &PasswordForm{}
	if err := c.Decode(form); err != nil {
		return nil, err
	}
	return form, nil
}

func (c *SessionsController) Create(form *PasswordForm) error {
	username := strings.TrimSpace(form.Username)
	if err := c.sessions.SignIn(username, form.Password); err != nil {
		return err
	}
	return c.RedirectToRoute(
		"admin",
		lazycontroller.RedirectStatus(http.StatusSeeOther),
	)
}
```

Keep request parsing in `GenPasswordForm`, not in `Create`.
`c.Decode(form)` parses the current request form through GoLazy's form decoder.
If decoding fails, return the error from the generator; GoLazy skips the action
and sends the error through the normal controller error path, including
`HandleError`.

Application validation errors stay in the action. On failed create or update,
return normal validation errors with `lazyerrors.Validator(form)`, set
`http.StatusUnprocessableEntity`, restore the form data needed by the view, and
render the form view. On success, flash if useful and redirect:

```go
type PostForm struct {
	Title   string `validate:"presence;min:3;max:120"`
	Content string `validate:"presence"`
}

func (c *PostsController) Create(form *PostForm) error {
	if err := lazyerrors.Validator(form); err != nil {
		c.Status(http.StatusUnprocessableEntity)
		c.Set("form", form)
		c.Set("post_errors", err)
		return c.Render("new")
	}
	post, err := c.posts.Create(form.Title, form.Content)
	if err != nil {
		return err
	}
	if err := c.FlashSet("notice", "Post created"); err != nil {
		return err
	}
	return c.RedirectToRoute(
		"post",
		post.ID,
		lazycontroller.RedirectStatus(http.StatusSeeOther),
	)
}
```

Templates can use `field_errors_for .post_errors "title"` for one field or
`errors_for .post_errors` for the full field-to-errors map.

## Expected Errors

Return `lazycontroller.Error(status, err)` for expected HTTP failures such as
missing records or forbidden actions. Unexpected errors become `500`.

When the condition comes from a helper or service, return a typed or sentinel
error and let the application base controller decide the response:

```go
var ErrNotFound = errors.New("not found")

func (c *BaseController) HandleError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) error {
	if errors.Is(err, ErrNotFound) {
		err = lazycontroller.Error(http.StatusNotFound, err)
	}
	return c.Base.HandleError(w, r, err)
}
```

Helpers should not render content or write `404` responses directly. They
should return errors such as `ErrNotFound`; `HandleError` owns the mapping to
status codes, redirects, or custom error views.

Add `app/views/app/error.html.tpl` only when the app needs to override the
framework default error page.

## Routes

Routes live in `init/routes.go`:

```go
func Draw(router *lazyroutes.Scope) {
	router.Get("/posts", postcontroller.New, (*postcontroller.PostsController).Index)
	router.Get("/posts/{post_id}", postcontroller.New, (*postcontroller.PostsController).Show)
}
```

Path parameters use the standard library `http.ServeMux` syntax and are read
with `r.PathValue("post_id")`.

Use `router.Resources(controller.New)` for conventional CRUD actions:

```text
GET    /posts                 Index
GET    /posts/new             New
POST   /posts                 Create
GET    /posts/{post_id}       Show
GET    /posts/{post_id}/edit  Edit
PATCH  /posts/{post_id}       Update
PUT    /posts/{post_id}       Update
DELETE /posts/{post_id}       Delete
```

`Resources` registers only actions that exist on the controller. Customize
resource path, singular, plural, parameter name, model mapping, collection
routes, member routes, or nested resources in the resource callback.

## Views And Layouts

Default controller views live at:

```text
app/views/<controller>/<action>.html.tpl
```

The default layout is:

```text
app/views/layouts/app.html.tpl
```

Template values come from controller `Set` calls:

```gotemplate
<h1>{{.title}}</h1>
```

Layouts receive the rendered controller view as `.content`.

Use `{{path_for "route_name"}}`, `{{stylesheet "/styles.css"}}`, and
`{{asset_path "/path"}}` instead of hard-coding generated paths.

Go templates escape values by default. Convert only trusted renderer output to
`template.HTML`.
