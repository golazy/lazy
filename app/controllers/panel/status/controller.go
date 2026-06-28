package status

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

type StatusController struct {
	panel.Base
}

func New(ctx context.Context) (*StatusController, error) {
	base, err := panel.NewBase(ctx)
	return &StatusController{Base: base}, err
}

func (c *StatusController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.SetState()
	return nil
}
