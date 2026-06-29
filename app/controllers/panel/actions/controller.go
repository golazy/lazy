package actions

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

type ActionsController struct {
	panel.Base
}

func New(ctx context.Context) (*ActionsController, error) {
	base, err := panel.NewBase(ctx)
	return &ActionsController{Base: base}, err
}

func (c *ActionsController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setActionsState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurbo(w, r, c.streamActions)
		},
	})
}

func (c *ActionsController) setActionsState(r *http.Request) {
	c.Set("state", c.Snapshot())
	c.Set("cache", c.CacheSnapshot(r.Context()))
}

func (c *ActionsController) streamActions(r *http.Request, _ buildservice.Event) (string, error) {
	body, err := c.RenderPanelPartial(r, "actions", "actions_frame", map[string]any{
		"state": c.Snapshot(),
		"cache": c.CacheSnapshot(r.Context()),
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStream("replace", "actions", body), nil
}
