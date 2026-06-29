package services

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"golazy.dev/lazy/app"
	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"

	_ "golazy.dev/lazyview/gotmpl"
)

func TestRestartEnqueuesServiceAction(t *testing.T) {
	store := buildservice.NewStore(10)
	store.UpdateService("postgres", buildservice.ServiceReady, "ready")
	actions := buildservice.NewActions()
	received := make(chan buildservice.ActionRequest, 1)
	go func() {
		request := <-actions
		received <- request
		request.Reply <- nil
	}()

	controller := &ServicesController{Base: panel.Base{Store: store, Actions: actions}}
	request := httptest.NewRequest(http.MethodPost, "/_golazy/services/postgres/restart", nil)
	request.SetPathValue("service_id", "postgres")
	response := httptest.NewRecorder()
	if err := controller.Restart(response, request); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusSeeOther, response.Body.String())
	}
	if got, want := response.Header().Get("Location"), "/_golazy/services?service=postgres"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
	action := <-received
	if action.Action != buildservice.ActionRestartService {
		t.Fatalf("action = %q, want %q", action.Action, buildservice.ActionRestartService)
	}
	if action.Service != "postgres" {
		t.Fatalf("service = %q, want postgres", action.Service)
	}
}

func TestRestartRejectsUnknownService(t *testing.T) {
	store := buildservice.NewStore(10)
	store.UpdateService("postgres", buildservice.ServiceReady, "ready")
	controller := &ServicesController{Base: panel.Base{Store: store, Actions: buildservice.NewActions()}}

	request := httptest.NewRequest(http.MethodPost, "/_golazy/services/minio/restart", nil)
	request.SetPathValue("service_id", "minio")
	response := httptest.NewRecorder()
	if err := controller.Restart(response, request); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestServiceOutputRowsFilterByTaskAndIncludeRun(t *testing.T) {
	events := []buildservice.Event{
		{Type: buildservice.EventOutput, Service: "postgres", Task: "migrate", Run: 1, Stream: "stdout", Output: "migrated\n"},
		{Type: buildservice.EventOutput, Service: "postgres", Task: "check", Run: 1, Stream: "stdout", Output: "NOT READY\n"},
		{Type: buildservice.EventOutput, Service: "postgres", Task: "check", Run: 2, Stream: "stdout", Output: "READY\n"},
		{Type: buildservice.EventOutput, Service: "postgres", Task: "start", Run: 1, Stream: "stderr", Output: "started\n"},
		{Type: buildservice.EventOutput, Service: "postgres", Task: "custom", Run: 1, Stream: "stdout", Output: "custom\n"},
	}

	tasks := serviceTasks(events, "postgres")
	if want := []string{"start", "check", "migrate", "custom"}; !reflect.DeepEqual(tasks, want) {
		t.Fatalf("tasks = %#v, want %#v", tasks, want)
	}

	rows := serviceOutputRows(events, "postgres", "check")
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want two check rows", rows)
	}
	if rows[0].Task != "check" || rows[0].RunLabel != "1" || rows[0].Message != "NOT READY" {
		t.Fatalf("first row = %#v, want check run 1 NOT READY", rows[0])
	}
	if rows[1].Task != "check" || rows[1].RunLabel != "2" || rows[1].Message != "READY" {
		t.Fatalf("second row = %#v, want check run 2 READY", rows[1])
	}

	filters := serviceTaskFilters("postgres", tasks, "check")
	if len(filters) != 5 || filters[0].Label != "All" || filters[0].Selected {
		t.Fatalf("filters = %#v, want unselected All plus task filters", filters)
	}
	if !filters[2].Selected || filters[2].URL != "/_golazy/services?service=postgres&task=check" {
		t.Fatalf("check filter = %#v, want selected check URL", filters[2])
	}
}

func TestServicesFrameRendersTaskFiltersAndRunColumn(t *testing.T) {
	events := []buildservice.Event{
		{Type: buildservice.EventOutput, Service: "postgres", Task: "check", Run: 1, Stream: "stdout", Output: "NOT READY\n"},
		{Type: buildservice.EventOutput, Service: "postgres", Task: "check", Run: 2, Stream: "stdout", Output: "READY\n"},
	}
	tasks := serviceTasks(events, "postgres")
	controller := &ServicesController{Base: panel.Base{}}
	controller.Renderer = newServicesTestRenderer(t)
	request := httptest.NewRequest(http.MethodGet, "/_golazy/services?service=postgres&task=check", nil)

	body, err := controller.RenderPanelPartial(request, "services", "services_frame", map[string]any{
		"state": buildservice.Snapshot{Services: []buildservice.ServiceSnapshot{{
			Name:  "postgres",
			State: buildservice.ServiceReady,
		}}},
		"selected_service":      "postgres",
		"selected_service_task": "check",
		"service_task_filters":  serviceTaskFilters("postgres", tasks, "check"),
		"service_output_rows":   serviceOutputRows(events, "postgres", "check"),
	})
	if err != nil {
		t.Fatalf("render services frame: %v", err)
	}
	for _, want := range []string{
		"<th>script</th>",
		"<th>run</th>",
		`href="/_golazy/services?service=postgres&amp;task=check"`,
		">1</td>",
		">2</td>",
		"NOT READY",
		"READY",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered services frame does not contain %q:\n%s", want, body)
		}
	}
}

func newServicesTestRenderer(t *testing.T) *lazycontroller.Renderer {
	t.Helper()
	views, err := app.Views()
	if err != nil {
		t.Fatalf("open app views: %v", err)
	}
	renderer, err := lazycontroller.NewRenderer(views)
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	renderer.Helper("path_for", func(name string, values ...any) (string, error) {
		if name == "restart_service" && len(values) > 0 {
			return "/_golazy/services/" + values[0].(string) + "/restart", nil
		}
		return "/_golazy/" + name, nil
	})
	return renderer
}
