# lazyshaft

Declare browser library entrypoints in `js.toml`. `lazy js` builds the
lazyshaft assets and importmap used by app-owned modules.

```toml
# js.toml
[entrypoint.turbo]
module = "@hotwired/turbo"

[entrypoint.stimulus]
module = "@hotwired/stimulus"
```

```sh
lazy js
```

```gotemplate
{{importmap "/assets/importmap.json"}}
<script type="module">import "app.js"</script>
```

Commit `app/public/assets/importmap.json` and
`app/public/assets/lazyshaft/`. Change `js.toml`, package files, or `app/js`
instead of editing generated files.

## Related

[Js](js.md) | [Assets](assets.md) | [Turbo/Visits](turbo-visits.md)
