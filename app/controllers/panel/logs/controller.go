package logs

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

type LogsController struct {
	panel.Base
}

func New(ctx context.Context) (*LogsController, error) {
	base, err := panel.NewBase(ctx)
	return &LogsController{Base: base}, err
}

func (c *LogsController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.SetState()
	return nil
}
