package logs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golazy.dev/lazy/app"
	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"

	_ "golazy.dev/lazyview/gotmpl"
)

func TestLogsStreamHydratesEventsAndPrependsNewRows(t *testing.T) {
	store := buildservice.NewStore(10)
	first := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	second := first.Add(time.Second)
	store.AddEvent(buildservice.Event{Type: buildservice.EventManual, Time: first, Message: "first"})
	store.AddEvent(buildservice.Event{Type: buildservice.EventReload, Time: second, Message: "second"})

	controller := &LogsController{Base: panel.Base{Store: store, Renderer: newLogsTestRenderer(t)}}
	request := httptest.NewRequest(http.MethodGet, "/_golazy/logs", nil)

	initial, err := controller.streamLogsInitial(request)
	if err != nil {
		t.Fatalf("stream initial logs: %v", err)
	}
	for _, want := range []string{
		`<turbo-stream action="update" target="panel_events">`,
		"<strong>reload</strong> second",
		"<strong>manual</strong> first",
	} {
		if !strings.Contains(initial, want) {
			t.Fatalf("initial stream missing %q:\n%s", want, initial)
		}
	}
	if strings.Index(initial, "second") > strings.Index(initial, "first") {
		t.Fatalf("initial stream did not render newest events first:\n%s", initial)
	}

	next, err := controller.streamLogs(request, buildservice.Event{Type: buildservice.EventOutput, Time: second.Add(time.Second), Output: "line"})
	if err != nil {
		t.Fatalf("stream new log: %v", err)
	}
	for _, want := range []string{
		`<turbo-stream action="prepend" target="panel_events">`,
		"<strong>output</strong> line",
	} {
		if !strings.Contains(next, want) {
			t.Fatalf("event stream missing %q:\n%s", want, next)
		}
	}
	if strings.Contains(next, `target="logs"`) {
		t.Fatalf("event stream replaced whole logs frame:\n%s", next)
	}
}

func newLogsTestRenderer(t *testing.T) *lazycontroller.Renderer {
	t.Helper()
	views, err := app.Views()
	if err != nil {
		t.Fatalf("open app views: %v", err)
	}
	renderer, err := lazycontroller.NewRenderer(views)
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	return renderer
}
