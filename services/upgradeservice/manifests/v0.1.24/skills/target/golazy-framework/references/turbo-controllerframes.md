# Turbo/ControllerFrames

Branch on `TurboFrame` when the same resource action serves a full page and a
single frame body.

```go
func (c *PostsController) Show(post *postservice.Post) error {
	c.Set("post", post)
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			return nil
		},
		lazycontroller.TurboFrame: func() error {
			return c.RenderTurboFrame("post")
		},
	})
}
```

Keep one `Show` route and one service lookup. Content negotiation chooses the
presentation.

## Related

[Turbo/Frames](turbo-frames.md) | [Turbo/Controller](turbo-controller.md) |
[Controllers/ContentTypes](controllers-contenttypes.md)
