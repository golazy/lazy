package dependencies

import (
	"context"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

type DependenciesController struct {
	panel.Base
}

func New(ctx context.Context) (*DependenciesController, error) {
	base, err := panel.NewBase(ctx)
	return &DependenciesController{Base: base}, err
}

func (c *DependenciesController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setDependenciesState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboWithInitial(w, r, c.streamDependenciesInitial, c.streamDependencies)
		},
	})
}

func (c *DependenciesController) setDependenciesState(r *http.Request) {
	for key, value := range c.dependenciesViewData(r) {
		c.Set(key, value)
	}
}

func (c *DependenciesController) streamDependenciesInitial(r *http.Request) (string, error) {
	return c.renderDependencies(r)
}

func (c *DependenciesController) streamDependencies(r *http.Request, event buildservice.Event) (string, error) {
	if event.Type != buildservice.EventState || event.State != buildservice.StateRunning {
		return "", nil
	}
	return c.renderDependencies(r)
}

func (c *DependenciesController) renderDependencies(r *http.Request) (string, error) {
	body, err := c.RenderPanelPartial(r, "dependencies", "dependencies_frame", c.dependenciesViewData(r))
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("replace", "[data-dependencies-panel]", body), nil
}

func (c *DependenciesController) dependenciesViewData(r *http.Request) map[string]any {
	graph := c.DependencyGraphSnapshot(r.Context())
	return map[string]any{
		"state":             c.Snapshot(),
		"dependencies":      graph,
		"dependency_nodes":  graph.NodeRows(),
		"dependency_edges":  graph.Edges,
		"dependency_counts": dependencyCounts{Services: graph.ServiceCount(), Edges: graph.EdgeCount()},
	}
}

type dependencyCounts struct {
	Services int
	Edges    int
}
