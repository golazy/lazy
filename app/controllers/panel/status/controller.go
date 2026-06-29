package status

import (
	"context"
	"fmt"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
	"golazy.dev/lazyturbo"
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
		lazycontroller.TurboFrame: func() error {
			return c.renderStatusFrame(w, r)
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurbo(w, r, c.streamStatus)
		},
	})
}

func (c *StatusController) setStatusState(r *http.Request) {
	for name, value := range c.statusVariables(r) {
		c.Set(name, value)
	}
}

func (c *StatusController) renderStatusFrame(w http.ResponseWriter, r *http.Request) error {
	if frameID := lazyturbo.FrameID(r); frameID != "status_bar" {
		return lazycontroller.Error(http.StatusBadRequest, fmt.Errorf("status frame %q is not available", frameID))
	}
	body, err := c.RenderPermanentPanelFrame(r, "status_bar", "status", "status_bar_frame", c.statusVariables(r))
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = w.Write([]byte(body))
	return err
}

func (c *StatusController) statusVariables(r *http.Request) map[string]any {
	return map[string]any{
		"state":      c.Snapshot(),
		"cache":      c.CacheSnapshot(r.Context()),
		"monitoring": c.RequestMonitoringSnapshot(r.Context()),
	}
}

func (c *StatusController) streamStatus(r *http.Request, event buildservice.Event) (string, error) {
	if event.Type != buildservice.EventState && event.Type != buildservice.EventManual {
		return "", nil
	}
	body, err := c.RenderPanelPartial(r, "status", "status_bar_content", c.statusVariables(r))
	if err != nil {
		return "", err
	}
	return panel.TurboStream("replace", "status_bar_content", body), nil
}
