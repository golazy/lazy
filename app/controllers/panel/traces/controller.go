package traces

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

const appRequestTracesPath = "/requests/traces"

type TracesController struct {
	panel.Base
}

func New(ctx context.Context) (*TracesController, error) {
	base, err := panel.NewBase(ctx)
	return &TracesController{Base: base}, err
}

func (c *TracesController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.RespondHTMLOrJSON(w, r, appRequestTracesPath)
}
