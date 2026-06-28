package console

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

type ConsoleController struct {
	panel.Base
}

func New(ctx context.Context) (*ConsoleController, error) {
	base, err := panel.NewBase(ctx)
	return &ConsoleController{Base: base}, err
}

func (c *ConsoleController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.SetState()
	return nil
}
