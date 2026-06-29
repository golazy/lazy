package services

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

type ServicesController struct {
	panel.Base
}

func New(ctx context.Context) (*ServicesController, error) {
	base, err := panel.NewBase(ctx)
	return &ServicesController{Base: base}, err
}

func (c *ServicesController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setServicesState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurbo(w, r, c.streamServices)
		},
	})
}

func (c *ServicesController) Restart(w http.ResponseWriter, r *http.Request) error {
	service := strings.TrimSpace(r.PathValue("service_id"))
	if service == "" {
		http.Error(w, "service name is required", http.StatusBadRequest)
		return nil
	}
	if !serviceExists(service, c.Snapshot()) {
		http.Error(w, "service not found", http.StatusNotFound)
		return nil
	}
	if err := c.Actions.EnqueueService(r.Context(), buildservice.ActionRestartService, service); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return nil
	}
	http.Redirect(w, r, "/_golazy/services?service="+url.QueryEscape(service), http.StatusSeeOther)
	return nil
}

func (c *ServicesController) setServicesState(r *http.Request) {
	state := c.Snapshot()
	selected := selectedService(r, state)
	c.Set("state", state)
	c.Set("selected_service", selected)
	c.Set("service_output_rows", serviceOutputRows(state.Events, selected))
}

func (c *ServicesController) streamServices(r *http.Request, _ buildservice.Event) (string, error) {
	state := c.Snapshot()
	selected := selectedService(r, state)
	body, err := c.RenderPanelPartial(r, "services", "services_frame", map[string]any{
		"state":               state,
		"selected_service":    selected,
		"service_output_rows": serviceOutputRows(state.Events, selected),
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStream("replace", "services", body), nil
}

type serviceOutputRow struct {
	Stream  string
	Time    string
	Message string
}

func selectedService(r *http.Request, state buildservice.Snapshot) string {
	selected := strings.TrimSpace(r.URL.Query().Get("service"))
	if selected != "" {
		for _, service := range state.Services {
			if service.Name == selected {
				return selected
			}
		}
	}
	if len(state.Services) == 0 {
		return ""
	}
	return state.Services[0].Name
}

func serviceExists(name string, state buildservice.Snapshot) bool {
	for _, service := range state.Services {
		if service.Name == name {
			return true
		}
	}
	return false
}

func serviceOutputRows(events []buildservice.Event, service string) []serviceOutputRow {
	if service == "" {
		return nil
	}
	rows := make([]serviceOutputRow, 0)
	for _, event := range events {
		if event.Type != buildservice.EventOutput || event.Service != service || event.Output == "" {
			continue
		}
		for _, line := range strings.Split(strings.ReplaceAll(event.Output, "\r\n", "\n"), "\n") {
			if line == "" {
				continue
			}
			rows = append(rows, serviceOutputRow{
				Stream:  event.Stream,
				Time:    event.Time.Local().Format("2006-01-02 15:04:05"),
				Message: line,
			})
		}
	}
	return rows
}
