package pwa

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

func TestPWAViewReadsApplicationControlPlaneStateAndWorkers(t *testing.T) {
	var gotPaths []string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.URL.Path {
		case appPWAPath:
			_, _ = fmt.Fprint(w, `{
				"enabled": true,
				"installable": true,
				"version": "v1.2.3",
				"manifest_path": "/manifest.webmanifest",
				"service_worker_name": "pwa",
				"service_worker_path": "/service-worker.js",
				"offline": {"enabled": true, "urls": 12, "fallback": "/offline"},
				"push": {"enabled": false}
			}`)
		case appWorkersPath:
			_, _ = fmt.Fprint(w, `{
				"workers": [
					{"name":"pwa","kind":"service_worker","path":"/service-worker.js","scope":"/","type":"classic","source":"generated","description":"GoLazy PWA service worker","pwa":true},
					{"name":"search","kind":"web_worker","path":"/assets/search-worker.js","type":"module","source":"asset"}
				]
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &PWAController{Base: panel.Base{Store: store}}
	request := httptest.NewRequest(http.MethodGet, "/_golazy/pwa?q=service", nil)

	data := controller.pwaViewData(request)
	if want := []string{http.MethodGet + " " + appPWAPath, http.MethodGet + " " + appWorkersPath}; len(gotPaths) != len(want) || gotPaths[0] != want[0] || gotPaths[1] != want[1] {
		t.Fatalf("proxied requests = %#v, want %#v", gotPaths, want)
	}
	if data["workers_total"] != 2 || data["workers_visible"] != 1 {
		t.Fatalf("worker counts = total %#v visible %#v, want 2/1", data["workers_total"], data["workers_visible"])
	}
	rows := data["workers"].([]workerRow)
	if len(rows) != 1 || rows[0].Name != "pwa" || rows[0].Owner != "PWA" || rows[0].Kind != "service_worker" {
		t.Fatalf("worker rows = %#v, want PWA service worker", rows)
	}

	renderer := newPWATestRenderer(t)
	controller.Renderer = renderer
	body, err := controller.RenderPanelPartial(request, "pwa", "pwa_frame", data)
	if err != nil {
		t.Fatalf("render PWA frame: %v", err)
	}
	for _, want := range []string{
		`data-controller="debounced-form"`,
		`data-debounced-form-frame-value="pwa_workers_table"`,
		`<turbo-frame id="pwa_workers_table" class="assets-table-frame">`,
		`<table class="data-grid assets-grid" data-controller="table-resize">`,
		`<tbody data-pwa-workers-list>`,
		`1 / 2 workers`,
		`v1.2.3`,
		`/manifest.webmanifest`,
		`/service-worker.js`,
		`GoLazy PWA service worker`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered PWA frame missing %q:\n%s", want, body)
		}
	}
}

func newPWATestRenderer(t *testing.T) *lazycontroller.Renderer {
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
