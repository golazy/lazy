package routes

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

type RoutesController struct {
	panel.Base
}

func New(ctx context.Context) (*RoutesController, error) {
	base, err := panel.NewBase(ctx)
	return &RoutesController{Base: base}, err
}

func (c *RoutesController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.SetState()
	return nil
}
