package buildinfo

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

type BuildInfoController struct {
	panel.Base
}

func New(ctx context.Context) (*BuildInfoController, error) {
	base, err := panel.NewBase(ctx)
	return &BuildInfoController{Base: base}, err
}

func (c *BuildInfoController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setBuildInfoState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboWithInitial(w, r, c.streamBuildInfoInitial, c.streamBuildInfo)
		},
	})
}

func (c *BuildInfoController) setBuildInfoState(r *http.Request) {
	for key, value := range c.buildInfoViewData(r) {
		c.Set(key, value)
	}
}

func (c *BuildInfoController) streamBuildInfoInitial(r *http.Request) (string, error) {
	return c.renderBuildInfo(r)
}

func (c *BuildInfoController) streamBuildInfo(r *http.Request, event buildservice.Event) (string, error) {
	if event.Type != buildservice.EventState || event.State != buildservice.StateRunning {
		return "", nil
	}
	return c.renderBuildInfo(r)
}

func (c *BuildInfoController) renderBuildInfo(r *http.Request) (string, error) {
	body, err := c.RenderPanelPartial(r, "buildinfo", "buildinfo_frame", c.buildInfoViewData(r))
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("replace", "[data-buildinfo-panel]", body), nil
}

func (c *BuildInfoController) buildInfoViewData(r *http.Request) map[string]any {
	return map[string]any{
		"state":     c.Snapshot(),
		"buildinfo": c.BuildInfoSnapshot(r.Context()),
	}
}
