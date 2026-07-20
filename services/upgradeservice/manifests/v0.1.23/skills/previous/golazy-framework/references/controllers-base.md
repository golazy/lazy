# Controllers/Base

Put shared presentation behavior, authorization generators, and HTTP error
mapping in `app/controllers/base_controller.go`. Do not move domain policy into
the base controller.

```go
package controllers

import (
	"context"
	"errors"
	"net/http"

	"golazy.dev/lazycontroller"
)

var ErrLoginRequired = errors.New("login required")

type BaseController struct {
	lazycontroller.Base
}

func NewBaseController(ctx context.Context) (BaseController, error) {
	base, err := lazycontroller.NewBase(ctx)
	if err != nil {
		return BaseController{}, err
	}
	return BaseController{Base: base}, nil
}

func (c *BaseController) HandleError(w http.ResponseWriter, r *http.Request, err error) error {
	if errors.Is(err, ErrLoginRequired) {
		return c.RedirectToRoute("login", lazycontroller.RedirectStatus(http.StatusSeeOther))
	}
	return c.Base.HandleError(w, r, err)
}
```

Keep typed domain errors in their service package. Map those errors to status
codes, redirects, or custom error views here when the mapping is app-wide.

## Related

[Controllers](controllers.md) | [Controllers/BeforeFilters](controllers-beforefilters.md)
| [Controllers/Session](controllers-session.md)
