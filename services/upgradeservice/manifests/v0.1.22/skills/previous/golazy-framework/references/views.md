# Views

Views are presentation. Keep domain decisions in services and prepare only the
values needed by the template in the controller.

## Paths

```text
app/views/layouts/app.html.tpl
app/views/<resource>/<action>.html.tpl
app/views/<resource>/_<partial>.html.tpl
app/views/<resource>/<action>.turbo_stream.tpl
```

An action that returns `nil` without writing a response renders its conventional
view. `Show` on `PostsController` renders `app/views/posts/show.html.tpl`.

```go
func (c *PostsController) Show(post *postservice.Post) error {
	c.Set("post", post)
	return nil
}
```

Use `Render` when one conventional action selects among a small, explicit set
of views. This is the preferred shape for page resources:

```go
func (c *PagesController) Show(pageID string) error {
	switch pageID {
	case "about", "pricing":
		c.Set("page", pageID)
		return c.Render(pageID)
	default:
		return lazycontroller.Error(http.StatusNotFound, fmt.Errorf("page %q not found", pageID))
	}
}
```

Use `Variants` when the logical view remains `show` but a specialized template
such as `show.html+featured.tpl` should be preferred. Do not accept an
unvalidated request value as an arbitrary template path.

## Template Data And Helpers

Use `Set("name", value)` for template data. Go templates escape values by
default. Convert only trusted framework-generated output to `template.HTML`.

```gotemplate
<h1>{{.post.Title}}</h1>
{{ partial "post_card" .post }}
<a href="{{path_for "post" .post.ID}}">Read</a>
{{stylesheet "/styles.css"}}
<img src="{{asset_path "/images/logo.svg"}}" alt="Logo">
```

Layouts receive the rendered action view as `.content`. Use helpers for named
routes and assets so routing and fingerprinting remain framework-owned.

## Related

[Controllers](controllers.md) | [Controllers/Variants](controllers-variants.md)
| [Forms](forms.md) | [Assets](assets.md) | [Turbo/Frames](turbo-frames.md)
