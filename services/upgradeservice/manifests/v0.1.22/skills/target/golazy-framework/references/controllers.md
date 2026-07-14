# Controllers

Controllers are request-local presentation adapters. They authorize, translate
request values, call services, choose response behavior, and set view data.
They do not own business rules.

```go
// app/controllers/post_controller/postcontroller.go
package postcontroller

import (
	"context"
	"fmt"

	"sample_app/app/controllers"
	"sample_app/services/postservice"
)

type PostsController struct {
	controllers.BaseController
	posts postservice.Service
}

func New(ctx context.Context) (*PostsController, error) {
	base, err := controllers.NewBaseController(ctx)
	if err != nil {
		return nil, err
	}
	posts, ok := postservice.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("post service is missing from application context")
	}
	return &PostsController{BaseController: base, posts: posts}, nil
}

func (c *PostsController) Show(post *postservice.Post) error {
	c.Set("post", post)
	return nil
}
```

Constructor rules:

- Accept only `context.Context` and return `(*Controller, error)`.
- Build `controllers.BaseController` first.
- Resolve required services through their typed `FromContext` helpers.
- Fail during construction when a required dependency is missing.
- Keep shared service instances concurrency-safe; keep render state on the
  request-local controller.

Action rules:

- Use conventional resource names before adding custom actions.
- Receive typed generator values rather than repeatedly parsing requests.
- Call one or more service operations, then set presentation values.
- Return `nil` for the conventional view, `Render` for another explicit view,
  or a redirect/content response helper.
- Return `lazycontroller.Error(status, err)` for expected HTTP failures.
- Never calculate domain outcomes in an action.

## Related

[Routes](routes.md) | [Controllers/Base](controllers-base.md) |
[Controllers/Generators](controllers-generators.md) | [Forms](forms.md) |
[Views](views.md) | [Services](services.md)
