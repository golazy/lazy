# SEO/href-lang-tags

Set the page language, canonical URL, and all language alternates together in
`Show`.

```go
func (c *PostsController) Show(post *postservice.Post) error {
	c.Language(post.Language)
	c.Canonical("https://example.com/" + post.Language + "/posts/" + post.Slug)
	c.Alternate("en", "https://example.com/en/posts/"+post.Slug)
	c.Alternate("de", "https://example.com/de/posts/"+post.Slug)
	c.Alternate("x-default", "https://example.com/posts/"+post.Slug)
	c.Set("post", post)
	return nil
}
```

Keep locale selection and available translations in a service when they depend
on business content. The controller only emits links for the result.

## Related

[SEO](seo.md) | [SEO/alternates](seo-alternates.md) |
[SEO/Metadata (JSONLD)](seo-metadata-jsonld.md)
