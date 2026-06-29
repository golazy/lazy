package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
	"golazy.dev/lazysse"
)

const appServiceName = "app"
const maxServiceOutputRows = 100

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
			return c.streamServicesBatched(w, r)
		},
	})
}

func (c *ServicesController) Restart(w http.ResponseWriter, r *http.Request) error {
	return c.enqueueServiceAction(w, r, buildservice.ActionRestartService)
}

func (c *ServicesController) Start(w http.ResponseWriter, r *http.Request) error {
	return c.enqueueServiceAction(w, r, buildservice.ActionStartService)
}

func (c *ServicesController) Stop(w http.ResponseWriter, r *http.Request) error {
	return c.enqueueServiceAction(w, r, buildservice.ActionStopService)
}

func (c *ServicesController) enqueueServiceAction(w http.ResponseWriter, r *http.Request, action buildservice.Action) error {
	service := strings.TrimSpace(r.PathValue("service_id"))
	if service == "" {
		http.Error(w, "service name is required", http.StatusBadRequest)
		return nil
	}
	if !serviceExists(service, c.Snapshot()) {
		http.Error(w, "service not found", http.StatusNotFound)
		return nil
	}
	if err := c.Actions.EnqueueService(r.Context(), action, service); err != nil {
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
	tasks := serviceTasks(state, selected)
	selectedTask := selectedServiceTask(r, tasks)
	rows := serviceOutputRows(state.Events, selected, selectedTask)
	return map[string]any{
		"state":                 state,
		"selected_service":      selected,
		"selected_service_task": selectedTask,
		"service_nodes":         serviceNodes(state, selected, selectedTask),
		"mise_tasks":            miseTaskNodes(state.Tasks),
		"services_stream_url":   serviceURL(selected, selectedTask),
		"service_output_rows":   rows,
		"service_output_count":  len(rows),
	}
}

func (c *ServicesController) streamServicesInitial(r *http.Request) (string, error) {
	variables := c.servicesViewData(r)
	body, err := c.RenderPanelPartial(r, "services", "service_output_rows", variables)
	if err != nil {
		return "", err
	}
	list, err := c.RenderPanelPartial(r, "services", "service_list", variables)
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("update", "[data-service-output]", body) +
		panel.TurboStreamTargets("update", "[data-service-list]", list) +
		panel.TurboStreamTargets("update", "[data-service-output-count]", strconv.Itoa(len(variables["service_output_rows"].([]serviceOutputRow)))+" messages"), nil
}

func (c *ServicesController) streamServicesBatched(_ http.ResponseWriter, r *http.Request) error {
	stream, err := c.SSEStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	events, unsubscribe := c.Store.Subscribe()
	defer unsubscribe()
	stream.Heartbeat(15 * time.Second)

	initial, err := c.streamServicesInitial(r)
	if err != nil {
		return err
	}
	if err := sendServiceTurboStream(stream, initial); err != nil {
		return err
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	dirty := false
	for {
		select {
		case <-stream.Done():
			return nil
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if serviceStreamEvent(event) {
				dirty = true
			}
		case <-ticker.C:
			if !dirty {
				continue
			}
			dirty = false
			body, err := c.streamServicesInitial(r)
			if err != nil || body == "" {
				continue
			}
			if err := sendServiceTurboStream(stream, body); err != nil {
				return err
			}
		}
	}
}

func sendServiceTurboStream(stream *lazysse.Stream, body string) error {
	if body == "" {
		return nil
	}
	return stream.Send(lazysse.Event{Data: []string{body}})
}

func serviceStreamEvent(event buildservice.Event) bool {
	switch event.Type {
	case buildservice.EventOutput, buildservice.EventState, buildservice.EventFileChange, buildservice.EventReload, buildservice.EventManual:
		return true
	default:
		return false
	}
}

func (c *ServicesController) streamServices(r *http.Request, event buildservice.Event) (string, error) {
	switch event.Type {
	case buildservice.EventOutput, buildservice.EventState, buildservice.EventFileChange, buildservice.EventReload:
		return c.streamServiceOutput(r, event)
	case buildservice.EventManual:
		return c.streamServiceOutput(r, event)
	default:
		return "", nil
	}
}

func (c *ServicesController) streamServiceOutput(r *http.Request, event buildservice.Event) (string, error) {
	state := c.Snapshot()
	selected := selectedService(r, state)
	tasks := serviceTasks(state, selected)
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
	list, err := c.RenderPanelPartial(r, "services", "service_list", map[string]any{
		"service_nodes": serviceNodes(state, selected, selectedTask),
		"mise_tasks":    miseTaskNodes(state.Tasks),
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("prepend", "[data-service-output]", body) +
		panel.TurboStreamTargets("update", "[data-service-list]", list) +
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
	Attrs    string
}

type serviceNode struct {
	Name     string
	Label    string
	URL      string
	State    string
	Message  string
	Selected bool
	App      bool
	Tasks    []serviceTaskNode
}

func (n serviceNode) Running() bool {
	return n.State != string(buildservice.ServiceStopped)
}

type serviceTaskNode struct {
	Label    string
	URL      string
	Selected bool
}

func selectedService(r *http.Request, state buildservice.Snapshot) string {
	selected := strings.TrimSpace(r.URL.Query().Get("service"))
	if selected == appServiceName {
		return appServiceName
	}
	if selected != "" && selected != appServiceName {
		for _, service := range state.Services {
			if service.Name == selected {
				return selected
			}
		}
	}
	return appServiceName
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

func serviceNodes(state buildservice.Snapshot, selectedService string, selectedTask string) []serviceNode {
	nodes := []serviceNode{{
		Name:     appServiceName,
		Label:    "App",
		URL:      serviceURL(appServiceName, ""),
		State:    string(appServiceState(state.State)),
		Message:  state.Message,
		Selected: selectedService == appServiceName && selectedTask == "",
		App:      true,
		Tasks:    serviceTaskNodes(appServiceName, serviceTasks(state, appServiceName), selectedService, selectedTask),
	}}
	for _, service := range state.Services {
		nodes = append(nodes, serviceNode{
			Name:     service.Name,
			Label:    service.Name,
			URL:      serviceURL(service.Name, ""),
			State:    string(service.State),
			Message:  service.Message,
			Selected: selectedService == service.Name && selectedTask == "",
			Tasks:    serviceTaskNodes(service.Name, serviceTasks(state, service.Name), selectedService, selectedTask),
		})
	}
	return nodes
}

func serviceTaskNodes(service string, tasks []string, selectedService string, selectedTask string) []serviceTaskNode {
	nodes := make([]serviceTaskNode, 0, len(tasks))
	for _, task := range tasks {
		nodes = append(nodes, serviceTaskNode{
			Label:    task,
			URL:      serviceURL(service, task),
			Selected: selectedService == service && selectedTask == task,
		})
	}
	return nodes
}

func miseTaskNodes(tasks []string) []serviceTaskNode {
	nodes := make([]serviceTaskNode, 0, len(tasks))
	for _, task := range tasks {
		nodes = append(nodes, serviceTaskNode{Label: task})
	}
	return nodes
}

func appServiceState(state buildservice.State) buildservice.ServiceState {
	switch state {
	case buildservice.StateRunning:
		return buildservice.ServiceReady
	case buildservice.StateStopped:
		return buildservice.ServiceStopped
	default:
		return buildservice.ServiceNotReady
	}
}

func serviceTasks(state buildservice.Snapshot, service string) []string {
	if service == "" {
		return nil
	}
	seen := map[string]struct{}{}
	if service == appServiceName {
		for _, event := range state.Events {
			if event.Service != "" {
				continue
			}
			if task := appEventTask(event); task != "" {
				seen[task] = struct{}{}
			}
		}
		for _, task := range []string{"rebuild", "restart", "lazy js", "lazy tailwind", "output", "state", "changes", "reload"} {
			seen[task] = struct{}{}
		}
	} else {
		prefix := service + ":"
		for _, task := range state.Tasks {
			if !strings.HasPrefix(task, prefix) {
				continue
			}
			action := strings.TrimPrefix(task, prefix)
			if action != "" {
				seen[action] = struct{}{}
			}
		}
		for _, event := range state.Events {
			if event.Type != buildservice.EventOutput || event.Service != service || event.Task == "" {
				continue
			}
			seen[event.Task] = struct{}{}
		}
	}
	return orderedServiceTasks(seen)
}

func orderedServiceTasks(seen map[string]struct{}) []string {
	preferred := []string{"start", "check", "create", "migrate", "rebuild", "restart", "lazy js", "lazy tailwind", "output", "state", "changes", "reload"}
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
		if !eventMatchesService(event, service) {
			continue
		}
		if task != "" && eventTask(event, service) != task {
			continue
		}
		rows = append(rows, eventOutputRows(event, service)...)
	}
	if len(rows) > maxServiceOutputRows {
		rows = rows[len(rows)-maxServiceOutputRows:]
	}
	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
	return rows
}

func eventMatchesService(event buildservice.Event, service string) bool {
	if service == appServiceName {
		return event.Service == ""
	}
	return event.Type == buildservice.EventOutput && event.Service == service
}

func eventOutputRows(event buildservice.Event, service string) []serviceOutputRow {
	task := eventTask(event, service)
	output := eventOutput(event, service)
	if output == "" {
		return nil
	}
	rows := make([]serviceOutputRow, 0)
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if line == "" {
			continue
		}
		message, attrs := parseServiceMessage(line)
		rows = append(rows, serviceOutputRow{
			Task:     taskLabel(task),
			Run:      event.Run,
			RunLabel: runLabel(event.Run),
			Stream:   streamLabel(event),
			Time:     event.Time.Local().Format("2006-01-02 15:04:05"),
			Message:  message,
			Attrs:    attrs,
		})
	}
	return rows
}

func eventTask(event buildservice.Event, service string) string {
	if service != appServiceName {
		return event.Task
	}
	return appEventTask(event)
}

func appEventTask(event buildservice.Event) string {
	switch event.Type {
	case buildservice.EventOutput:
		return "output"
	case buildservice.EventState:
		return "state"
	case buildservice.EventFileChange:
		return "changes"
	case buildservice.EventReload:
		return "reload"
	case buildservice.EventManual:
		return "state"
	default:
		return string(event.Type)
	}
}

func eventOutput(event buildservice.Event, service string) string {
	if event.Output != "" {
		return event.Output
	}
	if service != appServiceName {
		return event.Message
	}
	switch event.Type {
	case buildservice.EventState, buildservice.EventManual:
		return event.Message
	case buildservice.EventFileChange:
		if len(event.Changed) == 0 {
			return "Files changed"
		}
		return "Changed " + strings.Join(event.Changed, ", ")
	case buildservice.EventReload:
		if event.Message != "" {
			return event.Message
		}
		return "Browser reload broadcast."
	default:
		return event.Message
	}
}

func streamLabel(event buildservice.Event) string {
	if event.Stream != "" {
		return event.Stream
	}
	return string(event.Type)
}

func parseServiceMessage(line string) (string, string) {
	var record map[string]interface{}
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		return line, ""
	}
	message := ""
	for _, key := range []string{"message", "msg"} {
		if value, ok := record[key]; ok {
			message = fmt.Sprint(value)
			delete(record, key)
			break
		}
	}
	if message == "" {
		message = line
	}
	if len(record) == 0 {
		return message, ""
	}
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	attrs := make([]string, 0, len(keys))
	for _, key := range keys {
		attrs = append(attrs, key+"="+fmt.Sprint(record[key]))
	}
	return message, strings.Join(attrs, " ")
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
