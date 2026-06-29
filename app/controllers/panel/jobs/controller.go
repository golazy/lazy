package jobs

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
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
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.Set("state", c.Snapshot())
			c.Set("jobs", c.JobsSnapshot(r.Context()))
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurbo(w, r, c.streamJobs)
		},
	})
}

func (c *JobsController) streamJobs(r *http.Request, _ buildservice.Event) (string, error) {
	body, err := c.RenderPanelPartial(r, "jobs", "jobs_frame", map[string]any{
		"state": c.Snapshot(),
		"jobs":  c.JobsSnapshot(r.Context()),
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStream("replace", "jobs", body), nil
}
