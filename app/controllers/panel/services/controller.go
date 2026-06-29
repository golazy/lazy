package services

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
			c.Set("defer_panel_lists", true)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboWithInitial(w, r, c.streamServicesInitial, c.streamServices)
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
	for key, value := range c.servicesViewData(r) {
		c.Set(key, value)
	}
}

func (c *ServicesController) servicesViewData(r *http.Request) map[string]any {
	state := c.Snapshot()
	selected := selectedService(r, state)
	tasks := serviceTasks(state.Events, selected)
	selectedTask := selectedServiceTask(r, tasks)
	return map[string]any{
		"state":                 state,
		"selected_service":      selected,
		"selected_service_task": selectedTask,
		"service_task_filters":  serviceTaskFilters(selected, tasks, selectedTask),
		"services_stream_url":   serviceURL(selected, selectedTask),
		"service_output_rows":   serviceOutputRows(state.Events, selected, selectedTask),
	}
}

func (c *ServicesController) streamServicesInitial(r *http.Request) (string, error) {
	variables := c.servicesViewData(r)
	body, err := c.RenderPanelPartial(r, "services", "service_output_rows", variables)
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("update", "[data-service-output]", body), nil
}

func (c *ServicesController) streamServices(r *http.Request, event buildservice.Event) (string, error) {
	switch event.Type {
	case buildservice.EventOutput:
		return c.streamServiceOutput(r, event)
	case buildservice.EventManual:
		if event.Service == "" {
			return "", nil
		}
		return c.streamServiceList(r)
	default:
		return "", nil
	}
}

func (c *ServicesController) streamServiceOutput(r *http.Request, event buildservice.Event) (string, error) {
	state := c.Snapshot()
	selected := selectedService(r, state)
	tasks := serviceTasks(state.Events, selected)
	selectedTask := selectedServiceTask(r, tasks)
	rows := serviceOutputRows([]buildservice.Event{event}, selected, selectedTask)
	if len(rows) == 0 {
		return "", nil
	}
	body, err := c.RenderPanelPartial(r, "services", "service_output_rows", map[string]any{
		"service_output_rows": rows,
	})
	if err != nil {
		return "", err
	}
	filters, err := c.RenderPanelPartial(r, "services", "service_task_filters", map[string]any{
		"service_task_filters": serviceTaskFilters(selected, tasks, selectedTask),
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("prepend", "[data-service-output]", body) +
		panel.TurboStreamTargets("update", "[data-service-task-filter]", filters) +
		panel.TurboStreamTargets("update", "[data-service-output-count]", strconv.Itoa(len(serviceOutputRows(state.Events, selected, selectedTask)))+" messages"), nil
}

func (c *ServicesController) streamServiceList(r *http.Request) (string, error) {
	variables := c.servicesViewData(r)
	body, err := c.RenderPanelPartial(r, "services", "service_list", variables)
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("update", "[data-service-list]", body), nil
}

type serviceOutputRow struct {
	Task     string
	Run      int
	RunLabel string
	Stream   string
	Time     string
	Message  string
}

type serviceTaskFilter struct {
	Label    string
	URL      string
	Selected bool
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

func selectedServiceTask(r *http.Request, tasks []string) string {
	selected := strings.TrimSpace(r.URL.Query().Get("task"))
	if selected == "" {
		return ""
	}
	for _, task := range tasks {
		if task == selected {
			return selected
		}
	}
	return ""
}

func serviceTaskFilters(service string, tasks []string, selected string) []serviceTaskFilter {
	filters := []serviceTaskFilter{{
		Label:    "All",
		URL:      serviceURL(service, ""),
		Selected: selected == "",
	}}
	for _, task := range tasks {
		filters = append(filters, serviceTaskFilter{
			Label:    task,
			URL:      serviceURL(service, task),
			Selected: selected == task,
		})
	}
	return filters
}

func serviceURL(service string, task string) string {
	values := url.Values{}
	if service != "" {
		values.Set("service", service)
	}
	if task != "" {
		values.Set("task", task)
	}
	if encoded := values.Encode(); encoded != "" {
		return "/_golazy/services?" + encoded
	}
	return "/_golazy/services"
}

func serviceTasks(events []buildservice.Event, service string) []string {
	if service == "" {
		return nil
	}
	seen := map[string]struct{}{}
	for _, event := range events {
		if event.Type != buildservice.EventOutput || event.Service != service || event.Task == "" {
			continue
		}
		seen[event.Task] = struct{}{}
	}
	preferred := []string{"start", "check", "create", "migrate"}
	tasks := make([]string, 0, len(seen))
	for _, task := range preferred {
		if _, ok := seen[task]; ok {
			tasks = append(tasks, task)
			delete(seen, task)
		}
	}
	preferredCount := len(tasks)
	for task := range seen {
		tasks = append(tasks, task)
	}
	if len(tasks) > preferredCount {
		tail := tasks[preferredCount:]
		sort.Strings(tail)
	}
	return tasks
}

func serviceOutputRows(events []buildservice.Event, service string, task string) []serviceOutputRow {
	if service == "" {
		return nil
	}
	rows := make([]serviceOutputRow, 0)
	for _, event := range events {
		if event.Type != buildservice.EventOutput || event.Service != service || event.Output == "" {
			continue
		}
		if task != "" && event.Task != task {
			continue
		}
		for _, line := range strings.Split(strings.ReplaceAll(event.Output, "\r\n", "\n"), "\n") {
			if line == "" {
				continue
			}
			rows = append(rows, serviceOutputRow{
				Task:     taskLabel(event.Task),
				Run:      event.Run,
				RunLabel: runLabel(event.Run),
				Stream:   event.Stream,
				Time:     event.Time.Local().Format("2006-01-02 15:04:05"),
				Message:  line,
			})
		}
	}
	return rows
}

func taskLabel(task string) string {
	if task == "" {
		return "service"
	}
	return task
}

func runLabel(run int) string {
	if run <= 0 {
		return "-"
	}
	return strconv.Itoa(run)
}
