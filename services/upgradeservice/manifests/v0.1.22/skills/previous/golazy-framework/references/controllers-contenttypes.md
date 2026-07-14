# Controllers/ContentTypes

Register custom formats once and use `Wants` when one resource action supports
multiple representations. Keep the underlying data operation in a service.

```go
var Markdown = lazycontroller.NewFormat(
	"text/markdown",
	lazycontroller.As("markdown"),
	lazycontroller.Suffix("md", "markdown"),
)

func (c *PostsController) Show(w http.ResponseWriter, post *postservice.Post) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.Set("post", post)
			return nil
		},
		Markdown: func() error {
			c.ContentType("text/markdown; charset=utf-8")
			_, err := fmt.Fprintf(w, "# %s\n", post.Title)
			return err
		},
	})
}
```

Use the same `Show`, `Create`, or `Update` action across formats. Do not create
format-specific routes and actions when content negotiation expresses the
same resource operation.

## Related

[Controllers](controllers.md) | [Turbo/Controller](turbo-controller.md) |
[Routes](routes.md)
