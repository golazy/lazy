# Cache/Views

Cache expensive partials and frame bodies in templates. Keep request-specific
frame attributes outside the cached body.

```gotemplate
{{/* app/views/posts/index.html.tpl */}}
{{ range .posts }}
  {{ cache (cache_key "post-card" .ID .UpdatedAt) "post_card" . }}
{{ end }}
```

```gotemplate
{{/* app/views/posts/show.html.tpl */}}
{{ turbo_frame "post" .post (cache_key "post" .post.ID .post.UpdatedAt) (turbo_src .post.URL) }}
```

```gotemplate
{{/* app/views/posts/_post_card.html.tpl */}}
<article>
  <h2>{{.Title}}</h2>
  <p>{{.Summary}}</p>
</article>
```

The same key-safety rule as action caching applies: include every dimension
that can change or restrict the rendered content.

## Related

[Cache/Actions](cache-actions.md) | [Views](views.md) |
[Turbo/Frames](turbo-frames.md)
