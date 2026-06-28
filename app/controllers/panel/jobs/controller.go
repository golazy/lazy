package jobs

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
)

const appJobsPath = "/jobs"

type JobsController struct {
	panel.Base
}

func New(ctx context.Context) (*JobsController, error) {
	base, err := panel.NewBase(ctx)
	return &JobsController{Base: base}, err
}

func (c *JobsController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.RespondHTMLOrJSON(w, r, appJobsPath)
}
