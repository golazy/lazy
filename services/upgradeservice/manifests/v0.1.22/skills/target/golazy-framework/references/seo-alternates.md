# SEO/alternates

Use `AlternateLink` for alternate URLs that need media, MIME type, or title
attributes beyond `hreflang`.

```go
func (c *PostsController) Show(post *postservice.Post) error {
	c.SEO(lazyseo.AlternateLink(lazyseo.Alternate{
		Type:  "application/rss+xml",
		Title: "Posts feed",
		URL:   "https://example.com/posts/feed.xml",
	}))
	c.SEO(lazyseo.AlternateLink(lazyseo.Alternate{
		Media: "only screen and (max-width: 640px)",
		URL:   "https://m.example.com/posts/" + post.Slug,
	}))
	c.Set("post", post)
	return nil
}
```

## Related

[SEO](seo.md) | [SEO/href-lang-tags](seo-href-lang-tags.md) |
[SEO/Metadata (JSONLD)](seo-metadata-jsonld.md)
