package cache

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

func TestCacheViewReadsApplicationControlPlaneAndRendersSelection(t *testing.T) {
	var gotPaths []string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.URL.Path {
		case "/cache":
			_, _ = fmt.Fprint(w, `{
				"enabled":true,
				"stats":{"entries":2,"max_entries":20,"size_bytes":1048576,"hits":4,"misses":1,"sets":2,"evictions":0},
				"entries":[
					{"key":"build-test-post-1","size_bytes":18,"updated_at":"2026-06-29T12:00:00Z"},
					{"key":"build-test-home","size_bytes":8,"updated_at":"2026-06-29T12:00:00Z"}
				]
			}`)
		case "/cache/entry":
			_, _ = fmt.Fprint(w, `{"key":"build-test-post-1","size_bytes":18,"content":"<p>cached post</p>","content_type":"text/html; charset=utf-8"}`)
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
	controller := &CacheController{Base: panel.Base{Store: store, Renderer: newCacheTestRenderer(t)}}
	request := httptest.NewRequest(http.MethodGet, "/_golazy/cache?q=post&key=build-test-post-1", nil)
	snapshot := controller.CacheSnapshot(request.Context())
	detail := controller.CacheEntry(request.Context(), "build-test-post-1")
	view := cacheView{Snapshot: snapshot, Query: "post", SelectedKey: "build-test-post-1", Selected: detail}

	if got, want := strings.Join(gotPaths, ","), "/cache,/cache/entry?key=build-test-post-1"; got != want {
		t.Fatalf("proxied paths = %q, want %q", got, want)
	}
	if got, want := view.SizeText(), "1.0MB"; got != want {
		t.Fatalf("SizeText = %q, want %q", got, want)
	}
	if got, want := view.UsageText(), "10%"; got != want {
		t.Fatalf("UsageText = %q, want %q", got, want)
	}
	if rows := view.Entries(); len(rows) != 1 || rows[0].Key != "build-test-post-1" {
		t.Fatalf("filtered rows = %#v, want selected post key", rows)
	}

	body, err := controller.RenderPanelPartial(request, "cache", "cache_frame", map[string]any{
		"cache": view,
	})
	if err != nil {
		t.Fatalf("render cache frame: %v", err)
	}
	for _, want := range []string{
		`Size: 1.0MB`,
		`Usage: 10%`,
		`<table class="data-grid cache-key-grid" data-controller="table-resize">`,
		`build-test-post-1`,
		`&lt;p&gt;cached post&lt;/p&gt;`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered cache frame missing %q:\n%s", want, body)
		}
	}
}

func newCacheTestRenderer(t *testing.T) *lazycontroller.Renderer {
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
