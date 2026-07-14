# Forms

Forms are presentation input. Decode them with a typed generator, validate
request shape in the action, and pass accepted values to a service for business
validation and state changes.

```go
// app/controllers/post_controller/post_form.go
type PostForm struct {
	Title   string `validate:"presence;min:3;max:120"`
	Content string `validate:"presence"`
}

func (c *PostsController) GenPostForm() (*PostForm, error) {
	form := &PostForm{}
	if err := c.Decode(form); err != nil {
		return nil, err
	}
	return form, nil
}
```

```go
func (c *PostsController) New() error {
	c.Set("form", &PostForm{})
	return nil
}

func (c *PostsController) Create(form *PostForm) error {
	if err := lazyerrors.Validator(form); err != nil {
		c.Status(http.StatusUnprocessableEntity)
		c.Set("form", form)
		c.Set("post_errors", err)
		return c.Render("new")
	}

	post, err := c.posts.Create(form.Title, form.Content)
	if err != nil {
		return err
	}
	return c.RedirectToRoute(
		"post",
		post.ID,
		lazycontroller.RedirectStatus(http.StatusSeeOther),
	)
}
```

```gotemplate
{{/* app/views/posts/new.html.tpl */}}
{{ form_for .form . (form_action (path_for "posts")) (form_file "post_fields") }}
```

```gotemplate
{{/* app/views/posts/_post_fields.html.tpl */}}
{{ text_field "Title" }}
{{ textarea "Content" }}
{{ range field_errors_for .post_errors "title" }}<p>{{.}}</p>{{ end }}
{{ submit_button "Create post" }}
```

Decode failures follow `HandleError`. Presentation validation failures render
the form with `422`. Domain failures come from the service as typed errors.
Successful creates and updates redirect with `303 See Other`.

## Related

[Controllers/Generators](controllers-generators.md) | [Services](services.md) |
[Views](views.md) | [Turbo/Forms](turbo-forms.md)
