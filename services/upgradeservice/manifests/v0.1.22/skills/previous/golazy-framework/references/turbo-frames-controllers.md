# Turbo/Frames/Controllers

Return a frame directly when a resource endpoint exists only to fill or refresh
that frame.

```go
func (c *PostsController) Comments(post *postservice.Post) error {
	comments, err := c.posts.Comments(c.Request().Context(), post.ID)
	if err != nil {
		return err
	}
	c.Set("comments", comments)
	return c.RenderTurboFrame("comments", lazyturbo.Loading("lazy"))
}
```

```gotemplate
{{/* app/views/posts/_comments_frame.html.tpl */}}
<ol>
  {{ range .comments }}<li>{{.Body}}</li>{{ else }}<li>No comments yet.</li>{{ end }}
</ol>
```

Register it as a resource member or collection route, not an unrelated
top-level `Get`.

## Related

[Turbo/Frames](turbo-frames.md) | [Routes](routes.md) |
[Controllers/Generators](controllers-generators.md)
