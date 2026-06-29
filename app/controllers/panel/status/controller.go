package status

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

type StatusController struct {
	panel.Base
}

func New(ctx context.Context) (*StatusController, error) {
	base, err := panel.NewBase(ctx)
	return &StatusController{Base: base}, err
}

func (c *StatusController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setStatusState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurbo(w, r, c.streamStatus)
		},
	})
}

func (c *StatusController) setStatusState(r *http.Request) {
	c.Set("state", c.Snapshot())
	c.Set("cache", c.CacheSnapshot(r.Context()))
	c.Set("monitoring", c.RequestMonitoringSnapshot(r.Context()))
}

func (c *StatusController) streamStatus(r *http.Request, _ buildservice.Event) (string, error) {
	body, err := c.RenderPanelPartial(r, "status", "status_bar_content", map[string]any{
		"state":      c.Snapshot(),
		"cache":      c.CacheSnapshot(r.Context()),
		"monitoring": c.RequestMonitoringSnapshot(r.Context()),
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStream("replace", "status_bar_content", body), nil
}
