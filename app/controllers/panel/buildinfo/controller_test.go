package buildinfo

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
		BuildCount:       3,
		BuildTrace: buildservice.BuildTraceSummary{
			Available:   true,
			BuildNumber: 3,
			Total:       250 * time.Millisecond,
			Phases: []buildservice.BuildTracePhase{
				{Name: buildservice.BuildTracePhaseBuild, Duration: 200 * time.Millisecond, Count: 2},
				{Name: buildservice.BuildTracePhaseLink, Duration: 50 * time.Millisecond, Count: 1},
			},
			Packages: []buildservice.BuildTracePackage{
				{
					Package:  "example.test/app/pkg",
					Phase:    buildservice.BuildTracePhaseBuild,
					Duration: 200 * time.Millisecond,
					Count:    2,
				},
				{
					Package:  "example.test/app/pkg2",
					Phase:    buildservice.BuildTracePhaseBuild,
					Duration: 180 * time.Millisecond,
					Count:    1,
				},
				{
					Package:  "example.test/app/pkg3",
					Phase:    buildservice.BuildTracePhaseBuild,
					Duration: 160 * time.Millisecond,
					Count:    1,
				},
				{
					Package:  "example.test/app/pkg4",
					Phase:    buildservice.BuildTracePhaseBuild,
					Duration: 140 * time.Millisecond,
					Count:    1,
				},
				{
					Package:  "example.test/app/pkg5",
					Phase:    buildservice.BuildTracePhaseBuild,
					Duration: 120 * time.Millisecond,
					Count:    1,
				},
				{
					Package:  "example.test/app/pkg6",
					Phase:    buildservice.BuildTracePhaseBuild,
					Duration: 100 * time.Millisecond,
					Count:    1,
				},
			},
			Actions: []buildservice.BuildTraceAction{{
				Name:     "Executing action (build example.test/app/pkg)",
				Phase:    buildservice.BuildTracePhaseBuild,
				Package:  "example.test/app/pkg",
				Duration: 180 * time.Millisecond,
			}},
		},
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
		`Build 3 - 250.0ms`,
		`Build Phases`,
		`Top Packages`,
		`Runtime Details`,
		`Settings`,
		`Dependencies`,
		`example.test/app/pkg`,
		`title="example.test/app/pkg"`,
		`2 dependencies`,
		`example.test/app/cmd/app`,
		`go1.26.0`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered buildinfo frame missing %q:\n%s", want, body)
		}
	}
	for _, unwanted := range []string{
		`Slow Actions`,
		`Slowest Packages`,
		`example.test/app/pkg6`,
		`vcs.revision`,
		`../replaced`,
	} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("rendered buildinfo frame contains old panel heading %q:\n%s", unwanted, body)
		}
	}

	settingsRequest := httptest.NewRequest(http.MethodGet, "/_golazy/buildinfo?tab=settings", nil)
	settingsBody, err := controller.RenderPanelPartial(settingsRequest, "buildinfo", "buildinfo_frame", controller.buildInfoViewData(settingsRequest))
	if err != nil {
		t.Fatalf("render settings buildinfo frame: %v", err)
	}
	for _, want := range []string{
		`aria-current="page">Settings`,
		`GOOS`,
		`vcs.revision`,
		`abc123`,
	} {
		if !strings.Contains(settingsBody, want) {
			t.Fatalf("rendered settings buildinfo frame missing %q:\n%s", want, settingsBody)
		}
	}

	dependenciesRequest := httptest.NewRequest(http.MethodGet, "/_golazy/buildinfo?tab=dependencies", nil)
	dependenciesBody, err := controller.RenderPanelPartial(dependenciesRequest, "buildinfo", "buildinfo_frame", controller.buildInfoViewData(dependenciesRequest))
	if err != nil {
		t.Fatalf("render dependencies buildinfo frame: %v", err)
	}
	for _, want := range []string{
		`aria-current="page">Dependencies`,
		`golazy.dev`,
		`../replaced`,
	} {
		if !strings.Contains(dependenciesBody, want) {
			t.Fatalf("rendered dependencies buildinfo frame missing %q:\n%s", want, dependenciesBody)
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
