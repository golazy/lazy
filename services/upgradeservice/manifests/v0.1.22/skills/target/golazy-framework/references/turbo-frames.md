# Turbo/Frames

Render a frame helper in the full page and keep its underscored partial beside
the resource views.

```gotemplate
{{/* app/views/posts/show.html.tpl */}}
<section>
  <h1>{{.post.Title}}</h1>
  {{ turbo_frame "comments" . (turbo_src (path_for "post_comments" .post.ID)) (turbo_loading "lazy") }}
</section>
```

```gotemplate
{{/* app/views/posts/_comments_frame.html.tpl */}}
<ol>
  {{ range .comments }}
    <li>{{.Body}}</li>
  {{ else }}
    <li>No comments yet.</li>
  {{ end }}
</ol>
```

Use a resource collection/member route for the frame endpoint. Keep frame
loading in the controller and domain retrieval in a service.

## Related

[Turbo/Frames/Controllers](turbo-frames-controllers.md) |
[Turbo/ControllerFrames](turbo-controllerframes.md) | [Cache/Views](cache-views.md)
| [Routes](routes.md)
