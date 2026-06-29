package console

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

type ConsoleController struct {
	panel.Base
}

func New(ctx context.Context) (*ConsoleController, error) {
	base, err := panel.NewBase(ctx)
	return &ConsoleController{Base: base}, err
}

func (c *ConsoleController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.SetState()
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurbo(w, r, c.streamConsole)
		},
	})
}

func (c *ConsoleController) streamConsole(r *http.Request, _ buildservice.Event) (string, error) {
	body, err := c.RenderPanelPartial(r, "console", "console_frame", map[string]any{"state": c.Snapshot()})
	if err != nil {
		return "", err
	}
	return panel.TurboStream("replace", "console", body), nil
}
