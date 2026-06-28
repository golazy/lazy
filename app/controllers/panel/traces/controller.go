package traces

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

type TracesController struct {
	panel.Base
}

func New(ctx context.Context) (*TracesController, error) {
	base, err := panel.NewBase(ctx)
	return &TracesController{Base: base}, err
}

func (c *TracesController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.SetState()
	return nil
}
