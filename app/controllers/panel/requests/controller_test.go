package requests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golazy.dev/lazy/app"
	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"

	_ "golazy.dev/lazyview/gotmpl"
)

func TestRequestViewReadsTracesAndRendersRequestDetails(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotQuery url.Values
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{
			"directory":".tmp/traces",
			"traces":[{
				"request_id":"req-123",
				"method":"GET",
				"path":"/pools",
				"status":200,
				"category":"framework",
				"handled_by":"lazydispatch.Router",
				"bytes":2048,
				"duration_ms":12.5,
				"trace_file":".tmp/traces/req-123.trace",
				"runtime":{"go_version":"go1.26","goos":"linux","goarch":"amd64"},
				"memory":{"total_alloc_bytes_delta":4096,"mallocs_delta":12},
				"spans":[
					{"name":"http.server.request","span_id":"root","started_at":"2026-06-29T08:00:00Z","ended_at":"2026-06-29T08:00:00.012Z","duration_ms":12,"self_duration_ms":7},
					{"name":"controller pools#Index","span_id":"controller","parent_id":"root","started_at":"2026-06-29T08:00:00.001Z","ended_at":"2026-06-29T08:00:00.006Z","duration_ms":5,"self_duration_ms":2,"memory":{"total_alloc_bytes_delta":4096,"mallocs_delta":12,"frees_delta":3,"self_total_alloc_bytes_delta":1024,"self_mallocs_delta":4,"self_frees_delta":1}}
				],
				"logs":[{"time":"2026-06-29T08:00:00Z","level":"info","message":"handled","span_id":"controller"}]
			},{
				"request_id":"req-asset",
				"method":"GET",
				"path":"/assets/app.js",
				"status":200,
				"category":"assets",
				"handled_by":"lazyassets.Registry",
				"bytes":1024,
				"duration_ms":2.5
			}]
		}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &RequestsController{Base: panel.Base{Store: store}}

	request := httptest.NewRequest(http.MethodGet, "/_golazy/requests?q=pools&domain=lazydispatch.Router&request=req-123&span=controller&tab=tracing&framework=1", nil)
	view := controller.requestView(request)
	if gotMethod != http.MethodGet || gotPath != appRequestTracesPath {
		t.Fatalf("proxied request = %s %s, want GET %s", gotMethod, gotPath, appRequestTracesPath)
	}
	if gotQuery.Get("q") != "pools" || gotQuery.Get("type") != "" || gotQuery.Get("domain") != "" {
		t.Fatalf("proxied query = %s, want only q=pools", gotQuery.Encode())
	}
	if view.Error != "" {
		t.Fatalf("view error = %q", view.Error)
	}
	if !view.HasSelected || view.Selected.RequestID != "req-123" {
		t.Fatalf("selected request = %#v, want req-123", view.Selected)
	}
	if !view.TracingTab() {
		t.Fatalf("selected tab = %q, want tracing", view.Tab)
	}
	if len(view.Rows) != 1 || view.Rows[0].Trace.PathText() != "/pools" {
		t.Fatalf("rows = %#v, want /pools request row", view.Rows)
	}
	if len(view.DomainFilters) != 3 || view.DomainFilters[0].Label != "All" || view.DomainFilters[0].Selected || view.DomainFilters[1].Label != "lazyassets.Registry" || view.DomainFilters[2].Label != "lazydispatch.Router" || !view.DomainFilters[2].Selected {
		t.Fatalf("domain filters = %#v, want All then sorted domains with lazydispatch selected", view.DomainFilters)
	}
	allRequest := httptest.NewRequest(http.MethodGet, "/_golazy/requests?q=pools&request=req-123", nil)
	allView := controller.requestView(allRequest)
	if len(allView.DomainFilters) == 0 || allView.DomainFilters[0].Label != "All" || !allView.DomainFilters[0].Selected {
		t.Fatalf("default domain filters = %#v, want selected All filter", allView.DomainFilters)
	}
	if !view.HasSelectedSpan || view.SelectedSpan.SpanID != "controller" {
		t.Fatalf("selected span = %#v, want controller", view.SelectedSpan)
	}
	if got := view.SelectedSpan.AllocationSummaryText(); !strings.Contains(got, "4.0 KiB total") || !strings.Contains(got, "1.0 KiB self") {
		t.Fatalf("selected allocation summary = %q, want total and self allocations", got)
	}
	if got := view.SelectedSpan.FlameLabel(); got != "controller pools#Index" {
		t.Fatalf("selected flame label = %q, want span name", got)
	}
	if got := view.SelectedSpan.FlameTooltip(); !strings.Contains(got, "Span: controller pools#Index") || !strings.Contains(got, "Memory: 4.0 KiB total, 1.0 KiB self") {
		t.Fatalf("selected flame tooltip = %q, want full details", got)
	}
	if got := view.StreamURL(); !strings.Contains(got, "request=req-123") || !strings.Contains(got, "span=controller") || !strings.Contains(got, "tab=tracing") || !strings.Contains(got, "domain=lazydispatch.Router") || strings.Contains(got, "type=") {
		t.Fatalf("StreamURL = %q, want selected request/span/tab/domain and no type", got)
	}

	renderer := newRequestTestRenderer(t)
	controller.Renderer = renderer
	body, err := controller.RenderPanelPartial(request, "requests", "requests_frame", map[string]any{
		"state":      controller.Snapshot(),
		"monitoring": panel.RequestMonitoringSnapshot{Enabled: true, Directory: ".tmp/traces"},
		"requests":   view,
	})
	if err != nil {
		t.Fatalf("render requests frame: %v", err)
	}
	for _, want := range []string{
		"/pools",
		"lazydispatch.Router",
		"lazyassets.Registry",
		`data-turbo-frame="request_detail"`,
		`<turbo-frame id="request_detail" src="/_golazy/requests?`,
		`domain=lazydispatch.Router`,
		`aria-label="Request handlers"`,
		">All</a>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered requests frame does not contain %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "More filters") || strings.Contains(body, `aria-label="Request type filters"`) {
		t.Fatalf("rendered requests frame still contains removed category filter UI:\n%s", body)
	}

	streamBody, err := controller.streamRequestsInitial(request)
	if err != nil {
		t.Fatalf("stream requests initial: %v", err)
	}
	clearIndex := strings.Index(streamBody, `targets="[data-request-list]"><template></template>`)
	rowIndex := strings.Index(streamBody, `/pools`)
	if clearIndex < 0 || rowIndex < 0 || clearIndex > rowIndex {
		t.Fatalf("stream body should clear before hydrating rows:\n%s", streamBody)
	}

	frameRequest := httptest.NewRequest(http.MethodGet, "/_golazy/requests?q=pools&domain=lazydispatch.Router&request=req-123&span=controller&tab=tracing&framework=1", nil)
	frameRequest.Header.Set("Turbo-Frame", "request_detail")
	frameResponse := httptest.NewRecorder()
	if err := controller.renderRequestDetailFrame(frameResponse, frameRequest); err != nil {
		t.Fatalf("render request detail frame: %v", err)
	}
	detailBody := frameResponse.Body.String()
	for _, want := range []string{
		`<turbo-frame id="request_detail">`,
		"Tracing",
		"controller pools#Index",
		"12.5ms, 12 allocs, 4.0 KiB",
		"request-region-grid",
		"sort=memory-self",
		"4.0 KiB",
		"1.0 KiB",
		`title="Span: controller pools#Index`,
		"Memory: 4.0 KiB total, 1.0 KiB self",
		"trace-flame-color-",
	} {
		if !strings.Contains(detailBody, want) {
			t.Fatalf("rendered request detail frame does not contain %q:\n%s", want, detailBody)
		}
	}
}

func newRequestTestRenderer(t *testing.T) *lazycontroller.Renderer {
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
