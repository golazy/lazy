package assets

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
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
			return c.StreamTurbo(w, r, c.streamAssets)
		},
	})
}

func (c *AssetsController) streamAssets(r *http.Request, _ buildservice.Event) (string, error) {
	body, err := c.RenderPanelPartial(r, "assets", "assets_frame", map[string]any{"state": c.Snapshot()})
	if err != nil {
		return "", err
	}
	return panel.TurboStream("replace", "assets", body), nil
}
