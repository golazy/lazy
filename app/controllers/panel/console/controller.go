package console

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
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
			return c.StreamTurboInitial(w, r, nil)
		},
	})
}
