# Cache/Actions

Call `CacheKey` before setting expensive render data. A cache hit writes the
stored response, so the action should return immediately.

```go
func (c *PostsController) Show(post *postservice.Post) error {
	if c.CacheKey(post.ID, post.UpdatedAt) {
		return nil
	}
	c.Set("post", post)
	return nil
}

func (c *PostsController) Sidebar() error {
	updatedAt, err := c.posts.UpdatedAt(c.Request().Context())
	if err != nil {
		return err
	}
	if c.CacheKeyF("posts-sidebar", updatedAt) {
		return nil
	}
	posts, err := c.posts.Latest(c.Request().Context())
	if err != nil {
		return err
	}
	c.NoLayout()
	c.Set("posts", posts)
	return c.Render("sidebar")
}
```

Include every value that changes the representation in the key: record
version, locale, authorization scope, selected format, and relevant query
state. Do not cache private output under a shared key.

## Related

[Cache/Views](cache-views.md) | [Controllers/LazyLoading](controllers-lazyloading.md)
| [Services](services.md)
