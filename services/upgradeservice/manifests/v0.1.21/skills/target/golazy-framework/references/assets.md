# Assets

Embed views and public files through `app/files.go`. Link assets by logical
path so GoLazy can return fingerprinted permanent URLs in production.

```go
// app/files.go
package app

import (
	"embed"

	"golazy.dev/lazyapp"
)

//go:embed views public
var Files embed.FS

var Views = lazyapp.MustSub(Files, "views")
var Public = lazyapp.MustSub(Files, "public")
```

```go
func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{
		Name:         "sample_app",
		Drawer:       Draw,
		Public:       app.Public,
		Views:        app.Views,
		Dependencies: Dependencies,
	})
}
```

```gotemplate
{{stylesheet "/styles.css"}}
<img src="{{asset_path "/images/logo.svg"}}" alt="Logo">
```

Put hand-authored public files under `app/public`. Generated CSS, importmaps,
and lazyshaft output also live there and are committed. Run `lazy tailwind` or
`lazy js` when their sources change. Do not commit `node_modules`.

During `lazy` development, views and public files reload from disk. Production
builds serve the embedded files from the application binary.

## Related

[Tailwind](tailwind.md) | [lazyshaft](lazyshaft.md) | [Js](js.md) |
[Views](views.md)
