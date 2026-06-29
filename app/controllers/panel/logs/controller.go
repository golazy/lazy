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
			c.Set("defer_panel_lists", true)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboWithInitial(w, r, c.streamLogsInitial, c.streamLogs)
		},
	})
}

func (c *LogsController) streamLogsInitial(r *http.Request) (string, error) {
	body, err := c.RenderPanelPartial(r, "logs", "events", map[string]any{
		"events": newestEvents(c.Snapshot().Events),
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStream("update", "panel_events", body), nil
}

func (c *LogsController) streamLogs(r *http.Request, event buildservice.Event) (string, error) {
	body, err := c.RenderPanelPartialData(r, "logs", "event_item", event)
	if err != nil {
		return "", err
	}
	return panel.TurboStream("prepend", "panel_events", body), nil
}

func newestEvents(events []buildservice.Event) []buildservice.Event {
	reversed := make([]buildservice.Event, 0, len(events))
	for index := len(events) - 1; index >= 0; index-- {
		reversed = append(reversed, events[index])
	}
	return reversed
}
