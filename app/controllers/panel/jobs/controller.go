package jobs

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

const appJobsPath = "/jobs"

type Controller struct {
	panel.Base
}

func New(ctx context.Context) (*Controller, error) {
	base, err := panel.NewBase(ctx)
	return &Controller{Base: base}, err
}

func (c *Controller) Index(w http.ResponseWriter, r *http.Request) error {
	return c.RespondHTMLOrJSON(w, r, appJobsPath)
}
