package assets

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazycontroller"
)

type AssetsController struct {
	panel.Base
}

func New(ctx context.Context) (*AssetsController, error) {
	base, err := panel.NewBase(ctx)
	return &AssetsController{Base: base}, err
}

func (c *AssetsController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.SetState()
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboInitial(w, r, nil)
		},
	})
}
