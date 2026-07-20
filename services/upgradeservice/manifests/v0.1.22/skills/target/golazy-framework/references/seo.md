# SEO

SEO is presentation metadata. Configure site-wide defaults once, render them in
the layout, and set page values in the conventional controller action.

```go
// init/seo.go
func SEO(ctx context.Context) []lazyseo.Option {
	return []lazyseo.Option{
		lazyseo.SiteName("Sample App"),
		lazyseo.Description("A small GoLazy application."),
		lazyseo.Language("en"),
		lazyseo.Locale("en_US"),
		lazyseo.Type("website"),
	}
}
```

```go
// init/app.go
func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{
		Name:         "sample_app",
		Drawer:       Draw,
		Public:       app.Public,
		Views:        app.Views,
		Dependencies: Dependencies,
		SEO:          SEO,
	})
}
```

```gotemplate
{{/* app/views/layouts/app.html.tpl */}}
<html lang="{{seo_lang}}">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    {{seo}}
    {{stylesheet "/styles.css"}}
  </head>
  <body>{{.content}}</body>
</html>
```

```go
func (c *PostsController) Show(post *postservice.Post) error {
	c.Title(post.Title)
	c.Description(post.Summary)
	c.Canonical("https://example.com/posts/" + post.Slug)
	c.Set("post", post)
	return nil
}
```

The service supplies domain data; the controller turns it into page metadata.

## Related

[SEO/href-lang-tags](seo-href-lang-tags.md) |
[SEO/alternates](seo-alternates.md) |
[SEO/Metadata (JSONLD)](seo-metadata-jsonld.md) | [Controllers](controllers.md)
