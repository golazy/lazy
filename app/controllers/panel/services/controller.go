package services

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

type ServicesController struct {
	panel.Base
}

func New(ctx context.Context) (*ServicesController, error) {
	base, err := panel.NewBase(ctx)
	return &ServicesController{Base: base}, err
}

func (c *ServicesController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.SetState()
	return nil
}
