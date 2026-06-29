package requests

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

type RequestsController struct {
	panel.Base
}

func New(ctx context.Context) (*RequestsController, error) {
	base, err := panel.NewBase(ctx)
	return &RequestsController{Base: base}, err
}

func (c *RequestsController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.Set("state", c.Snapshot())
			c.Set("monitoring", c.RequestMonitoringSnapshot(r.Context()))
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurbo(w, r, c.streamRequests)
		},
	})
}

func (c *RequestsController) streamRequests(r *http.Request, _ buildservice.Event) (string, error) {
	body, err := c.RenderPanelPartial(r, "requests", "requests_frame", map[string]any{
		"state":      c.Snapshot(),
		"monitoring": c.RequestMonitoringSnapshot(r.Context()),
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStream("replace", "requests", body), nil
}
