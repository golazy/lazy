# Turbo/Streams/Controller

Branch on `TurboStream` when one conventional create or update action supports
both full-page and stream responses.

```go
func (c *PostsController) Create(form *PostForm) error {
	post, err := c.posts.Create(c.Request().Context(), form.Title)
	if err != nil {
		return err
	}
	c.Set("post", post)
	c.Set("form", &PostForm{})
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			return c.RedirectToRoute(
				"post",
				post.ID,
				lazycontroller.RedirectStatus(http.StatusSeeOther),
			)
		},
		lazycontroller.TurboStream: func() error {
			return c.Render("create")
		},
	})
}
```

Do not create a second route or duplicate the service operation for Turbo.

## Related

[Turbo/Streams](turbo-streams.md) | [Turbo/Controller](turbo-controller.md) |
[Forms](forms.md)
