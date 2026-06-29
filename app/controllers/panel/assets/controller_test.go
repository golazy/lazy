package assets

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

func TestAssetsViewReadsApplicationControlPlaneManifest(t *testing.T) {
	var gotPath string
	var gotMethod string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{
			"assets":[
				{"path":"/assets/app.js","permanent":"/assets/app-123.js","content_type":"application/javascript","size":2048,"source":"lazy js","generated":true},
				{"path":"/styles.css","content_type":"text/css","size":6,"source":"public"}
			]
		}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &AssetsController{Base: panel.Base{Store: store}}
	request := httptest.NewRequest(http.MethodGet, "/_golazy/assets?q=app", nil)

	data := controller.assetsViewData(request)
	if gotMethod != http.MethodGet || gotPath != appAssetsPath {
		t.Fatalf("proxied request = %s %s, want GET %s", gotMethod, gotPath, appAssetsPath)
	}
	if data["assets_total"] != 2 || data["assets_visible"] != 1 {
		t.Fatalf("asset counts = total %#v visible %#v, want 2/1", data["assets_total"], data["assets_visible"])
	}
	rows := data["assets"].([]assetRow)
	if len(rows) != 1 || rows[0].Path != "/assets/app.js" || rows[0].Permanent != "/assets/app-123.js" || rows[0].Kind != "Generated" {
		t.Fatalf("asset rows = %#v, want generated app asset", rows)
	}

	renderer := newAssetsTestRenderer(t)
	controller.Renderer = renderer
	body, err := controller.RenderPanelPartial(request, "assets", "assets_frame", data)
	if err != nil {
		t.Fatalf("render assets frame: %v", err)
	}
	for _, want := range []string{
		`<table class="data-grid assets-grid">`,
		`<tbody data-assets-list>`,
		`/assets/app.js`,
		`/assets/app-123.js`,
		`Generated`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered assets frame missing %q:\n%s", want, body)
		}
	}
}

func newAssetsTestRenderer(t *testing.T) *lazycontroller.Renderer {
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
