package media

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

func TestMediaViewReadsApplicationControlPlaneSnapshot(t *testing.T) {
	var gotPath string
	var gotMethod string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{
			"storages":[{"name":"local","default":true,"readable":true,"writable":true,"deletable":true,"listable":true,"urlable":true}],
			"selected_storage":"local",
			"storage_objects":[{"key":"uploads/avatar.png","content_type":"image/png","size":2048,"url":"https://cdn.example.test/uploads/avatar.png"}],
			"files":[{"id":"file-1","filename":"avatar.png","content_type":"image/png","size":2048,"location":{"storage":"local","key":"uploads/avatar.png"},"url":"/files/file-1","variants":[{"source_file_id":"file-1","variant_key":"thumb","output_file_id":"file-2","output_url":"/files/file-2","status":"ready"}]}]
		}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &MediaController{Base: panel.Base{Store: store}}
	request := httptest.NewRequest(http.MethodGet, "/_golazy/media?q=avatar&storage=local&file=file-1", nil)

	view := controller.mediaView(request)
	if gotMethod != http.MethodGet || gotPath != "/media?q=avatar&storage=local&file=file-1" {
		t.Fatalf("proxied request = %s %s, want GET /media with query", gotMethod, gotPath)
	}
	if view.CountText() != "1 / 1 files" {
		t.Fatalf("CountText = %q, want filtered count", view.CountText())
	}
	if !view.HasSelectedFile() || view.SelectedFile.ID != "file-1" || len(view.SelectedFile.Variants) != 1 {
		t.Fatalf("selected file = %#v, want file-1 with variant", view.SelectedFile)
	}
	if len(view.Objects) != 1 || view.Objects[0].PublicURL == "" {
		t.Fatalf("objects = %#v, want object with public URL", view.Objects)
	}

	controller.Renderer = newMediaTestRenderer(t)
	body, err := controller.RenderPanelPartial(request, "media", "media_frame", map[string]any{
		"state": store.Snapshot(),
		"media": view,
	})
	if err != nil {
		t.Fatalf("render media frame: %v", err)
	}
	for _, want := range []string{
		`<section id="media" class="tool-view is-active media-view" data-view="media">`,
		`<form method="post" action="/_golazy/media/file/upload?redirect=`,
		`<table class="data-grid media-file-grid" data-controller="table-resize">`,
		`avatar.png`,
		`<span class="media-variant-badge" title="thumb">`,
		`href="/_golazy/media/download?file=file-1"`,
		`uploads/avatar.png`,
		`https://cdn.example.test/uploads/avatar.png`,
		`<table class="data-grid media-variant-grid" data-controller="table-resize">`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered media frame missing %q:\n%s", want, body)
		}
	}
}

func newMediaTestRenderer(t *testing.T) *lazycontroller.Renderer {
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
