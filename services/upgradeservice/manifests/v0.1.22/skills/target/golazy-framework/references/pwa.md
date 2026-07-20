# PWA

Opt in at application startup, render the helper in the layout, and keep the
offline cache explicit.

```go
func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{
		Name:   "sample_app",
		Drawer: Draw,
		Public: app.Public,
		Views:  app.Views,
		PWA: lazypwa.Config{
			Installable: true,
			Manifest: lazypwa.ManifestConfig{
				Name:       "Sample App",
				ShortName:  "Sample",
				StartURL:   "/",
				Scope:      "/",
				Display:    "standalone",
				ThemeColor: "#FDDD00",
			},
			Offline: lazypwa.OfflineConfig{
				Enabled:       true,
				FallbackURL:   "/",
				URLs:          []string{"/"},
				IncludeAssets: true,
			},
		},
	})
}
```

```gotemplate
<head>
  {{seo}}
  {{pwa}}
</head>
```

Cache only public, safe responses. Do not make authenticated or mutable domain
work depend on the browser being online unless the service and sync design
explicitly support it.

## Related

[Assets](assets.md) | [Js](js.md) | [SEO](seo.md) | [Cache/Actions](cache-actions.md)
