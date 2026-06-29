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
			c.Set("defer_panel_lists", true)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboWithInitial(w, r, c.streamJobsInitial, c.streamJobs)
		},
	})
}

func (c *JobsController) streamJobsInitial(r *http.Request) (string, error) {
	return c.renderJobsLists(r)
}

func (c *JobsController) streamJobs(r *http.Request, event buildservice.Event) (string, error) {
	if event.Type != buildservice.EventState || event.State != buildservice.StateRunning {
		return "", nil
	}
	return c.renderJobsLists(r)
}

func (c *JobsController) renderJobsLists(r *http.Request) (string, error) {
	variables := map[string]any{
		"state": c.Snapshot(),
		"jobs":  c.JobsSnapshot(r.Context()),
	}
	definitions, err := c.RenderPanelPartial(r, "jobs", "job_definitions", variables)
	if err != nil {
		return "", err
	}
	recent, err := c.RenderPanelPartial(r, "jobs", "recent_jobs", variables)
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("update", "[data-job-definitions]", definitions) +
		panel.TurboStreamTargets("update", "[data-jobs-recent]", recent), nil
}
