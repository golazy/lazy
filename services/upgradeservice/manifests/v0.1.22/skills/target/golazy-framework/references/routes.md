# Routes

Declare routes in `init/routes.go`. Start with resources; add explicit verb
routes only when the endpoint has no meaningful resource shape.

## Contents

- [Prefer Resources](#prefer-resources)
- [Model Pages As A Resource](#model-pages-as-a-resource)
- [Customize A Resource](#customize-a-resource)
- [Explicit Route Exceptions](#explicit-route-exceptions)

## Prefer Resources

`Resources` derives paths, names, parameters, controller metadata, and the
conventional action mapping. It registers only actions that the controller
actually implements.

```go
// init/routes.go
package appinit

import (
	"golazy.dev/lazyroutes"
	homecontroller "sample_app/app/controllers/home_controller"
	pagecontroller "sample_app/app/controllers/page_controller"
	postcontroller "sample_app/app/controllers/post_controller"
)

func Draw(router *lazyroutes.Scope) {
	router.Resources(homecontroller.New, func(home *lazyroutes.Resource) {
		home.Singular("home")
		home.Plural("home")
		home.Path("")
	})

	router.Resources(pagecontroller.New)
	router.Resources(postcontroller.New)
}
```

For `PostsController`, the available conventional methods map to:

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

Do not reproduce this table with separate `router.Get`, `router.Post`, and
other calls. Let the controller's conventional methods define what exists.

## Model Pages As A Resource

Do not create `About`, `Pricing`, and `Terms` actions with one `Get` route each
just to render different templates. Use a page resource and select the view in
`Show`:

```go
func (c *PagesController) Show(pageID string) error {
	switch pageID {
	case "about", "pricing", "terms":
		return c.Render(pageID)
	default:
		return lazycontroller.Error(http.StatusNotFound, fmt.Errorf("page %q not found", pageID))
	}
}
```

That keeps routing conventional (`GET /pages/{page_id}`) while the controller
owns presentation selection. Use `Variants` instead when the logical view is
still `show`.

## Customize A Resource

```go
router.Resources(postcontroller.New, func(posts *lazyroutes.Resource) {
	posts.Path("articles")
	posts.Singular("article")
	posts.Plural("articles")
	posts.Param("slug")
	posts.Model(postservice.Post{})

	posts.Get("search", (*postcontroller.PostsController).Search)
	posts.MemberPatch("publish", (*postcontroller.PostsController).Publish)
	posts.Resources(commentcontroller.New)
})
```

Use resource collection methods (`Get`, `Post`, and so on) for operations on
the collection, member methods (`MemberGet`, `MemberPatch`, and so on) for one
record, and nested `Resources` for child records.

## Explicit Route Exceptions

Use a top-level verb route for a truly non-resource controller endpoint such as
an OAuth callback. Use `HandleFunc` for a tiny raw endpoint that does not need a
controller, rendering, or app presentation state.

```go
router.HandleFunc(http.MethodGet, "/up", func(w http.ResponseWriter, _ *http.Request) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
})
```

Inspect the final table with `lazy routes` and make sure custom routes have not
replaced conventional resource routes without a real semantic reason.

## Related

[Controllers](controllers.md) | [Controllers/Generators](controllers-generators.md)
| [Views](views.md) | [Forms](forms.md)
