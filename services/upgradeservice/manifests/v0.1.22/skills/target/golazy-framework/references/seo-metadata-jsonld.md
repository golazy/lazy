# SEO/Metadata (JSONLD)

Use `Metadata` when one value can provide title, description, canonical URL,
social image, page kind, timestamps, and JSON-LD together. Keep business data
in the service and place presentation adaptation beside the controller when it
would otherwise make the domain model depend on SEO packages.

```go
// app/controllers/post_controller/post_metadata.go
type PostMetadata struct {
	Post postservice.Post
}

func (m PostMetadata) Title() string          { return m.Post.Title }
func (m PostMetadata) Description() string    { return m.Post.Summary }
func (m PostMetadata) Canonical() string      { return "https://example.com/posts/" + m.Post.Slug }
func (m PostMetadata) Image() string          { return m.Post.ImageURL }
func (m PostMetadata) Kind() lazyseo.PageKind { return lazyseo.Article }
func (m PostMetadata) PublishedTime() time.Time { return m.Post.PublishedAt }
func (m PostMetadata) LastUpdated() time.Time   { return m.Post.UpdatedAt }

func (m PostMetadata) JSONLD() any {
	article := jsonld.NewArticle(m.Post.Title)
	article.Description = m.Post.Summary
	article.URL = m.Canonical()
	article.Image = m.Post.ImageURL
	article.DatePublished = jsonld.Date(m.Post.PublishedAt)
	article.DateModified = jsonld.Date(m.Post.UpdatedAt)
	return article
}
```

```go
func (c *PostsController) Show(post *postservice.Post) error {
	c.Metadata(PostMetadata{Post: *post})
	c.Set("post", post)
	return nil
}
```

It is also acceptable for a domain model to implement these small methods when
the application deliberately treats canonical metadata as part of that model.
Do not move content selection or publication rules into the adapter.

## Related

[SEO](seo.md) | [Controllers](controllers.md) | [Services](services.md)
