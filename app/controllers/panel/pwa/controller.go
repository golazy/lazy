package pwa

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazycontroller"
	"golazy.dev/lazypwa"
	"golazy.dev/lazyturbo"
	"golazy.dev/lazyworkers"
)

const (
	appPWAPath     = "/pwa"
	appWorkersPath = "/workers"
)

type PWAController struct {
	panel.Base
}

func New(ctx context.Context) (*PWAController, error) {
	base, err := panel.NewBase(ctx)
	return &PWAController{Base: base}, err
}

func (c *PWAController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setPWAState(r)
			c.Set("defer_panel_lists", true)
			return nil
		},
		lazycontroller.TurboFrame: func() error {
			return c.renderPWATableFrame(w, r)
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboInitial(w, r, c.streamPWAInitial)
		},
	})
}

func (c *PWAController) setPWAState(r *http.Request) {
	for key, value := range c.pwaViewData(r) {
		c.Set(key, value)
	}
}

func (c *PWAController) streamPWAInitial(r *http.Request) (string, error) {
	variables := c.pwaViewData(r)
	body, err := c.RenderPanelPartial(r, "pwa", "worker_rows", variables)
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("update", "[data-pwa-workers-list]", body) +
		panel.TurboStreamTargets("update", "[data-pwa-workers-count]", workerCountText(variables)), nil
}

func (c *PWAController) renderPWATableFrame(w http.ResponseWriter, r *http.Request) error {
	if frameID := lazyturbo.FrameID(r); frameID != "pwa_workers_table" {
		return lazycontroller.Error(http.StatusBadRequest, fmt.Errorf("PWA frame %q is not available", frameID))
	}
	variables := c.pwaViewData(r)
	body, err := c.RenderPanelFrame(r, "pwa_workers_table", "pwa", "workers_table", variables)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = w.Write([]byte(body))
	return err
}

func (c *PWAController) pwaViewData(r *http.Request) map[string]any {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	data := map[string]any{
		"state":           c.Snapshot(),
		"pwa_query":       query,
		"pwa_stream":      pwaStreamURL(query),
		"pwa_state":       lazypwa.State{},
		"workers":         []workerRow{},
		"workers_total":   0,
		"workers_visible": 0,
	}

	var pwaState lazypwa.State
	if err := c.FetchAppControlJSON(r.Context(), appPWAPath, &pwaState); err != nil {
		data["pwa_error"] = err.Error()
	} else {
		data["pwa_state"] = pwaState
	}

	var workers lazyworkers.Manifest
	if err := c.FetchAppControlJSON(r.Context(), appWorkersPath, &workers); err != nil {
		data["workers_error"] = err.Error()
	} else {
		rows := workerRows(workers.Workers)
		filtered := filterWorkers(rows, query)
		data["workers"] = filtered
		data["workers_total"] = len(rows)
		data["workers_visible"] = len(filtered)
	}
	data["workers_count_text"] = workerCountText(data)
	return data
}

type workerRow struct {
	Name        string
	Kind        string
	Path        string
	Scope       string
	Type        string
	Source      string
	Owner       string
	Description string
}

func workerRows(workers []lazyworkers.Worker) []workerRow {
	rows := make([]workerRow, 0, len(workers))
	for _, worker := range workers {
		owner := "App"
		if worker.PWA {
			owner = "PWA"
		}
		rows = append(rows, workerRow{
			Name:        worker.Name,
			Kind:        string(worker.Kind),
			Path:        worker.Path,
			Scope:       worker.Scope,
			Type:        string(worker.Type),
			Source:      string(worker.Source),
			Owner:       owner,
			Description: worker.Description,
		})
	}
	return rows
}

func filterWorkers(workers []workerRow, query string) []workerRow {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return workers
	}
	filtered := make([]workerRow, 0, len(workers))
	for _, worker := range workers {
		if strings.Contains(strings.ToLower(strings.Join([]string{
			worker.Name,
			worker.Kind,
			worker.Path,
			worker.Scope,
			worker.Type,
			worker.Source,
			worker.Owner,
			worker.Description,
		}, " ")), query) {
			filtered = append(filtered, worker)
		}
	}
	return filtered
}

func workerCountText(variables map[string]any) string {
	if variables["workers_error"] != nil {
		return "Workers unavailable"
	}
	total, _ := variables["workers_total"].(int)
	visible, _ := variables["workers_visible"].(int)
	query, _ := variables["pwa_query"].(string)
	if query != "" {
		return strconv.Itoa(visible) + " / " + strconv.Itoa(total) + " workers"
	}
	return strconv.Itoa(total) + " workers"
}

func pwaStreamURL(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "/_golazy/pwa"
	}
	return "/_golazy/pwa?q=" + url.QueryEscape(query)
}
