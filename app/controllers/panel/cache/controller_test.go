package cache

import (
	"fmt"
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
					{"key":"build-test-post-1","size_bytes":18,"updated_at":"2026-06-29T12:00:00Z","hits":3,"sets":1},
					{"key":"build-test-home","size_bytes":8,"updated_at":"2026-06-29T12:00:00Z","hits":1,"sets":1}
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
		`Size: <span data-cache-size>1.0MB</span>`,
		`Usage: <span data-cache-usage>10%</span>`,
		`<table class="data-grid cache-key-grid" data-controller="table-resize">`,
		`<th data-table-resize-min-width-value="48">Hits</th>`,
		`build-test-post-1`,
		`>3</td>`,
		`&lt;p&gt;cached post&lt;/p&gt;`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered cache frame missing %q:\n%s", want, body)
		}
	}

	known := map[string]struct{}{}
	stream, err := controller.renderCacheEventStream(request, cacheEvent{
		Enabled: true,
		Stats: panel.CacheStats{
			Entries:    3,
			MaxEntries: 20,
			SizeBytes:  2097152,
			Hits:       5,
			Misses:     1,
			Sets:       3,
		},
		Entry: &panel.CacheEntry{
			Key:       "build-test-post-2",
			SizeBytes: 32,
			UpdatedAt: time.Now(),
			Hits:      1,
			Sets:      1,
		},
	}, known)
	if err != nil {
		t.Fatalf("render cache event stream: %v", err)
	}
	for _, want := range []string{
		`targets="[data-cache-size]"`,
		`2.0MB`,
		`targets="[data-cache-list]"`,
		`build-test-post-2`,
		`<td>1</td>`,
	} {
		if !strings.Contains(stream, want) {
			t.Fatalf("cache event stream missing %q:\n%s", want, stream)
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
