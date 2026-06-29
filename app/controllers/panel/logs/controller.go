package logs

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

type LogsController struct {
	panel.Base
}

func New(ctx context.Context) (*LogsController, error) {
	base, err := panel.NewBase(ctx)
	return &LogsController{Base: base}, err
}

func (c *LogsController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.SetState()
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurbo(w, r, c.streamLogs)
		},
	})
}

func (c *LogsController) streamLogs(r *http.Request, _ buildservice.Event) (string, error) {
	body, err := c.RenderPanelPartial(r, "logs", "logs_frame", map[string]any{
		"state": c.Snapshot(),
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStream("replace", "logs", body), nil
}
