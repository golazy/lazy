package traces

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

type Controller struct {
	panel.Base
}

func New(ctx context.Context) (*Controller, error) {
	base, err := panel.NewBase(ctx)
	return &Controller{Base: base}, err
}

func (c *Controller) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.SetState()
	return nil
}
