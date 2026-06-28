package actions

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

type ActionsController struct {
	panel.Base
}

func New(ctx context.Context) (*ActionsController, error) {
	base, err := panel.NewBase(ctx)
	return &ActionsController{Base: base}, err
}

func (c *ActionsController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.SetState()
	return nil
}
