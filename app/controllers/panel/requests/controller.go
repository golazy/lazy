package requests

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

type RequestsController struct {
	panel.Base
}

func New(ctx context.Context) (*RequestsController, error) {
	base, err := panel.NewBase(ctx)
	return &RequestsController{Base: base}, err
}

func (c *RequestsController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.SetState()
	return nil
}
