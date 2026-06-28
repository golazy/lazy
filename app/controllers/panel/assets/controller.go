package assets

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

type AssetsController struct {
	panel.Base
}

func New(ctx context.Context) (*AssetsController, error) {
	base, err := panel.NewBase(ctx)
	return &AssetsController{Base: base}, err
}

func (c *AssetsController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.SetState()
	return nil
}
