# Turbo/Controller

Use `Wants` to keep HTML, Turbo Frame, and Turbo Stream behavior in one
conventional action.

```go
func (c *PostsController) Update(post *postservice.Post, form *PostForm) error {
	updated, err := c.posts.Update(c.Request().Context(), post.ID, form.Title)
	if err != nil {
		return err
	}
	c.Set("post", updated)
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			return c.RedirectToRoute(
				"post",
				updated.ID,
				lazycontroller.RedirectStatus(http.StatusSeeOther),
			)
		},
		lazycontroller.TurboFrame: func() error {
			return c.RenderTurboFrame("post")
		},
		lazycontroller.TurboStream: func() error {
			return c.Render("update")
		},
	})
}
```

Perform the service operation once before choosing a representation. Keep
representation-specific rendering inside the format callbacks.

## Related

[Controllers/ContentTypes](controllers-contenttypes.md) |
[Turbo/ControllerFrames](turbo-controllerframes.md) |
[Turbo/Streams/Controller](turbo-streams-controller.md) | [Forms](forms.md)
