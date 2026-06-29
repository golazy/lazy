package traces

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golazy.dev/lazy/app"
	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"

	_ "golazy.dev/lazyview/gotmpl"
)

func TestTraceViewReadsApplicationControlPlaneAndPreservesSelectionURLs(t *testing.T) {
	var gotMethod string
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{
			"directory":".tmp/traces",
			"traces":[{
				"request_id":"req-123",
				"method":"GET",
				"path":"/pools",
				"status":200,
				"duration_ms":12.5,
				"trace_file":".tmp/traces/req-123.trace",
				"spans":[
					{"name":"http.server.request","span_id":"root","started_at":"2026-06-29T08:00:00Z","ended_at":"2026-06-29T08:00:00.012Z","duration_ms":12,"self_duration_ms":7},
					{"name":"controller pools#Index","span_id":"controller","parent_id":"root","started_at":"2026-06-29T08:00:00.001Z","ended_at":"2026-06-29T08:00:00.006Z","duration_ms":5,"self_duration_ms":2,"memory":{"total_alloc_bytes_delta":4096,"mallocs_delta":12,"frees_delta":3,"self_total_alloc_bytes_delta":1024,"self_mallocs_delta":4,"self_frees_delta":1}}
				],
				"logs":[{"time":"2026-06-29T08:00:00Z","level":"info","message":"handled","span_id":"controller"}]
			}]
		}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &TracesController{Base: panel.Base{Store: store}}

	request := httptest.NewRequest(http.MethodGet, "/_golazy/traces?q=pools&trace=req-123&span=controller&framework=1", nil)
	view := controller.traceView(request)
	if gotMethod != http.MethodGet || gotPath != appRequestTracesPath {
		t.Fatalf("proxied request = %s %s, want GET %s", gotMethod, gotPath, appRequestTracesPath)
	}
	if view.Error != "" {
		t.Fatalf("view error = %q", view.Error)
	}
	if !view.HasSelected || view.Selected.RequestID != "req-123" {
		t.Fatalf("selected trace = %#v, want req-123", view.Selected)
	}
	if !view.HasSelectedSpan || view.SelectedSpan.SpanID != "controller" {
		t.Fatalf("selected span = %#v, want controller", view.SelectedSpan)
	}
	if len(view.TimelineRows) != 2 || len(view.FlameRows) != 1 || len(view.Logs) != 1 {
		t.Fatalf("rows = timeline %d flame %d logs %d, want 2/1/1", len(view.TimelineRows), len(view.FlameRows), len(view.Logs))
	}
	if got := view.SelectedSpan.DurationSummaryText(); !strings.Contains(got, "5.00ms total") || !strings.Contains(got, "2.00ms self") {
		t.Fatalf("selected duration summary = %q, want total and self", got)
	}
	if got := view.SelectedSpan.AllocationSummaryText(); !strings.Contains(got, "4.0 KiB total") || !strings.Contains(got, "1.0 KiB self") {
		t.Fatalf("selected allocation summary = %q, want total and self allocations", got)
	}
	if got := view.SelectedSpan.MallocsSummaryText(); got != "12 mallocs total, 4 mallocs self" {
		t.Fatalf("selected malloc summary = %q", got)
	}
	if got := view.SelectedSpan.FlameLabel(); !strings.Contains(got, "self alloc 1.0 KiB") {
		t.Fatalf("selected flame label = %q, want self allocation", got)
	}
	renderer := newTraceTestRenderer(t)
	controller.Renderer = renderer
	body, err := controller.RenderPanelPartial(request, "traces", "traces_frame", map[string]any{
		"state":      controller.Snapshot(),
		"monitoring": panel.RequestMonitoringSnapshot{Enabled: true, Directory: ".tmp/traces"},
		"traces":     view,
	})
	if err != nil {
		t.Fatalf("render traces frame: %v", err)
	}
	for _, want := range []string{
		"5.00ms total, 2.00ms self",
		"4.0 KiB total, 1.0 KiB self",
		"12 mallocs total, 4 mallocs self",
		"self alloc 1.0 KiB",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered traces frame does not contain %q:\n%s", want, body)
		}
	}
	if got := view.StreamURL(); !strings.Contains(got, "trace=req-123") || !strings.Contains(got, "span=controller") || !strings.Contains(got, "framework=1") {
		t.Fatalf("StreamURL = %q, want selected trace/span/framework", got)
	}
}

func newTraceTestRenderer(t *testing.T) *lazycontroller.Renderer {
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
		return "/_golazy/" + name, nil
	})
	return renderer
}
