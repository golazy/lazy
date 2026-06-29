package services

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"golazy.dev/lazy/app"
	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"

	_ "golazy.dev/lazyview/gotmpl"
)

func TestRestartEnqueuesServiceAction(t *testing.T) {
	testServiceAction(t, http.MethodPost, "/_golazy/services/postgres/restart", "postgres", buildservice.ActionRestartService, (*ServicesController).Restart)
}

func TestStartEnqueuesServiceAction(t *testing.T) {
	testServiceAction(t, http.MethodPost, "/_golazy/services/postgres/start", "postgres", buildservice.ActionStartService, (*ServicesController).Start)
}

func TestStopEnqueuesServiceAction(t *testing.T) {
	testServiceAction(t, http.MethodPost, "/_golazy/services/postgres/stop", "postgres", buildservice.ActionStopService, (*ServicesController).Stop)
}

func testServiceAction(t *testing.T, method string, path string, service string, action buildservice.Action, handler func(*ServicesController, http.ResponseWriter, *http.Request) error) {
	t.Helper()
	store := buildservice.NewStore(10)
	store.UpdateService(service, buildservice.ServiceReady, "ready")
	actions := buildservice.NewActions()
	received := make(chan buildservice.ActionRequest, 1)
	go func() {
		request := <-actions
		received <- request
		request.Reply <- nil
	}()

	controller := &ServicesController{Base: panel.Base{Store: store, Actions: actions}}
	request := httptest.NewRequest(method, path, nil)
	request.SetPathValue("service_id", service)
	response := httptest.NewRecorder()
	if err := handler(controller, response, request); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusSeeOther, response.Body.String())
	}
	if got, want := response.Header().Get("Location"), "/_golazy/services?service="+service; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
	receivedAction := <-received
	if receivedAction.Action != action {
		t.Fatalf("action = %q, want %q", receivedAction.Action, action)
	}
	if receivedAction.Service != service {
		t.Fatalf("service = %q, want %s", receivedAction.Service, service)
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

	state := buildservice.Snapshot{
		Tasks:  []string{"postgres:start", "postgres:check", "postgres:migrate"},
		Events: events,
	}
	tasks := serviceTasks(state, "postgres")
	if want := []string{"start", "check", "migrate", "custom"}; !reflect.DeepEqual(tasks, want) {
		t.Fatalf("tasks = %#v, want %#v", tasks, want)
	}

	rows := serviceOutputRows(events, "postgres", "check")
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want two check rows", rows)
	}
	if rows[0].Task != "check" || rows[0].RunLabel != "2" || rows[0].Message != "READY" {
		t.Fatalf("first row = %#v, want check run 2 READY", rows[0])
	}
	if rows[1].Task != "check" || rows[1].RunLabel != "1" || rows[1].Message != "NOT READY" {
		t.Fatalf("second row = %#v, want check run 1 NOT READY", rows[1])
	}
}

func TestServiceOutputRowsCapAndParseJSON(t *testing.T) {
	events := make([]buildservice.Event, 0, maxServiceOutputRows+5)
	for i := 1; i <= maxServiceOutputRows+5; i++ {
		events = append(events, buildservice.Event{
			Type:    buildservice.EventOutput,
			Service: "postgres",
			Task:    "start",
			Run:     i,
			Stream:  "stdout",
			Output:  `{"message":"ready","port":3000,"run":` + strconv.Itoa(i) + "}\n",
		})
	}

	rows := serviceOutputRows(events, "postgres", "start")
	if len(rows) != maxServiceOutputRows {
		t.Fatalf("rows = %d, want %d", len(rows), maxServiceOutputRows)
	}
	if rows[0].RunLabel != strconv.Itoa(maxServiceOutputRows+5) || rows[0].Message != "ready" {
		t.Fatalf("first row = %#v, want newest parsed JSON row", rows[0])
	}
	if !strings.Contains(rows[0].Attrs, "port=3000") || !strings.Contains(rows[0].Attrs, "run=") {
		t.Fatalf("attrs = %q, want parsed JSON attributes", rows[0].Attrs)
	}
}

func TestServicesFrameRendersTaskTreeAndRunColumn(t *testing.T) {
	events := []buildservice.Event{
		{Type: buildservice.EventOutput, Service: "postgres", Task: "check", Run: 1, Stream: "stdout", Output: "NOT READY\n"},
		{Type: buildservice.EventOutput, Service: "postgres", Task: "check", Run: 2, Stream: "stdout", Output: "READY\n"},
	}
	state := buildservice.Snapshot{
		Services: []buildservice.ServiceSnapshot{{
			Name:  "postgres",
			State: buildservice.ServiceReady,
		}},
		Tasks:  []string{"postgres:start", "postgres:check", "lint"},
		Events: events,
	}
	controller := &ServicesController{Base: panel.Base{}}
	controller.Renderer = newServicesTestRenderer(t)
	request := httptest.NewRequest(http.MethodGet, "/_golazy/services?service=postgres&task=check", nil)

	body, err := controller.RenderPanelPartial(request, "services", "services_frame", map[string]any{
		"state":                 state,
		"selected_service":      "postgres",
		"selected_service_task": "check",
		"service_nodes":         serviceNodes(state, "postgres", "check"),
		"mise_tasks":            miseTaskNodes(state.Tasks),
		"service_output_rows":   serviceOutputRows(events, "postgres", "check"),
	})
	if err != nil {
		t.Fatalf("render services frame: %v", err)
	}
	for _, want := range []string{
		"<th>source</th>",
		"<th>run</th>",
		`href="/_golazy/services?service=postgres&amp;task=check"`,
		`action="/_golazy/services/postgres/restart"`,
		`action="/_golazy/services/postgres/stop"`,
		`aria-current="page"`,
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
		if (name == "restart_service" || name == "start_service" || name == "stop_service") && len(values) > 0 {
			action := strings.TrimSuffix(name, "_service")
			return "/_golazy/services/" + values[0].(string) + "/" + action, nil
		}
		return "/_golazy/" + name, nil
	})
	return renderer
}
