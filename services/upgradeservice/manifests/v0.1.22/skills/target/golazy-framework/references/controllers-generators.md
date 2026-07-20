# Controllers/Generators

Use `GenX` methods to turn request-derived values into typed action arguments.
Generators may receive request values, route parameters, context, and values
from other generators. GoLazy caches generated values by type for the current
request.

```go
func (c *PostsController) GenPost(postID int) (*postservice.Post, error) {
	post, err := c.posts.Find(postID)
	if errors.Is(err, postservice.ErrNotFound) {
		return nil, lazycontroller.Error(http.StatusNotFound, err)
	}
	return post, err
}

func (c *PostsController) Show(post *postservice.Post) error {
	c.Set("post", post)
	return nil
}
```

Name the generator after its result type. Keep decoding and loading mechanics
in the generator, but keep business validation and state transitions in the
service. A generator error skips hooks and actions that require the value and
enters the normal controller error path.

## Related

[Routes](routes.md) | [Controllers/BeforeFilters](controllers-beforefilters.md)
| [Forms](forms.md) | [Services](services.md)
