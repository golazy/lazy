# Turbo/Forms

Target a stable frame ID so validation errors and successful replacement
markup stay in the same region.

```gotemplate
{{/* app/views/posts/new.html.tpl */}}
{{ turbo_frame "post_form" . }}
```

```gotemplate
{{/* app/views/posts/_post_form_frame.html.tpl */}}
<form action="{{path_for "posts"}}" method="post" data-turbo-frame="post_form">
  <label for="post_title">Title</label>
  <input id="post_title" name="title" value="{{.form.Title}}">
  {{ range field_errors_for .post_errors "title" }}<p>{{.}}</p>{{ end }}
  <button type="submit">Create post</button>
</form>
```

Keep the normal `422` validation path and conventional create/update service
calls. Turbo changes the representation, not the business workflow.

## Related

[Forms](forms.md) | [Turbo/Frames](turbo-frames.md) |
[Turbo/Controller](turbo-controller.md)
