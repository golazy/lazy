# Controllers/LazyLoading

Use deferred view values only for presentation data that may be expensive.
The callback should call a service; it should not contain business logic.

```go
func (c *PostsController) Show(post *postservice.Post) error {
	c.Set("post", post)

	// Start now and resolve when the renderer needs the value.
	c.SetLater("related_posts", func(ctx context.Context) ([]postservice.Post, error) {
		return c.posts.Related(ctx, post.ID)
	})

	// Start only if a template reads this value.
	c.SetWhenNeeded("comment_count", func(ctx context.Context) (int, error) {
		return c.posts.CommentCount(ctx, post.ID)
	})

	return nil
}
```

Use `SetLater` where older code may say `SetEventually`. Preserve request
context cancellation and ensure the service method is concurrency-safe.

## Related

[Controllers](controllers.md) | [Views](views.md) | [Services](services.md) |
[Cache/Actions](cache-actions.md)
