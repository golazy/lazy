package buildinfo

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

func TestBuildInfoViewReadsApplicationControlPlane(t *testing.T) {
	var gotMethod string
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{
			"available": true,
			"go_version": "go1.26.0",
			"path": "example.test/app/cmd/app",
			"main": {"path":"example.test/app","version":"(devel)"},
			"deps": [
				{"path":"golazy.dev","version":"v0.1.17","sum":"h1:abc"},
				{"path":"example.test/replaced","version":"v1.0.0","replace":{"path":"../replaced"}}
			],
			"settings": [
				{"key":"GOOS","value":"linux"},
				{"key":"vcs.revision","value":"abc123"}
			]
		}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &BuildInfoController{Base: panel.Base{Store: store}}
	request := httptest.NewRequest(http.MethodGet, "/_golazy/buildinfo", nil)

	data := controller.buildInfoViewData(request)
	if gotMethod != http.MethodGet || gotPath != "/buildinfo" {
		t.Fatalf("proxied request = %s %s, want GET /buildinfo", gotMethod, gotPath)
	}
	snapshot := data["buildinfo"].(panel.BuildInfoSnapshot)
	if snapshot.Error != "" {
		t.Fatalf("snapshot error = %q", snapshot.Error)
	}
	if snapshot.GoVersion != "go1.26.0" || snapshot.Path != "example.test/app/cmd/app" {
		t.Fatalf("snapshot = %#v, want go version and command path", snapshot)
	}
	if len(snapshot.Deps) != 2 || snapshot.Deps[1].Replace == nil || snapshot.Deps[1].ReplaceText() != "../replaced" {
		t.Fatalf("deps = %#v, want replaced module", snapshot.Deps)
	}

	controller.Renderer = newBuildInfoTestRenderer(t)
	body, err := controller.RenderPanelPartial(request, "buildinfo", "buildinfo_frame", data)
	if err != nil {
		t.Fatalf("render buildinfo frame: %v", err)
	}
	for _, want := range []string{
		`data-buildinfo-panel`,
		`BuildInfo`,
		`2 dependencies`,
		`example.test/app/cmd/app`,
		`go1.26.0`,
		`vcs.revision`,
		`golazy.dev`,
		`../replaced`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered buildinfo frame missing %q:\n%s", want, body)
		}
	}
}

func newBuildInfoTestRenderer(t *testing.T) *lazycontroller.Renderer {
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
