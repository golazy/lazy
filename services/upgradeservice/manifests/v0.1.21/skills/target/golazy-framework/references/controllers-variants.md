# Controllers/Variants

Use variants when one conventional action keeps the same logical view but
prefers a specialized template.

```go
func (c *PostsController) Show(post *postservice.Post) error {
	if post.Featured {
		c.Variants("featured")
	}
	c.Set("post", post)
	return nil
}
```

```text
app/views/posts/show.html.tpl
app/views/posts/show.html+featured.tpl
app/views/posts/_card.html.tpl
app/views/posts/_card.html+featured.tpl
```

Use `Render("about")` instead when a page resource intentionally selects a
different logical view. Keep the set of allowed views explicit.

## Related

[Views](views.md) | [Routes](routes.md) | [Controllers](controllers.md)
