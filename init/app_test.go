package appinit

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golazy.dev/lazy/services/buildservice"
)

func TestPanelRootRedirectsToLogs(t *testing.T) {
	app := testApp()

	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy", nil))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusSeeOther, response.Body.String())
	}
	if got, want := response.Header().Get("Location"), "/_golazy/logs"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestPanelTabPageLoadsImportmapNavAndPermanentStatus(t *testing.T) {
	app := testApp()

	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy/logs", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{
		`<script type="importmap">`,
		`"/js/app.js": "/_golazy/assets/lazyshaft/app/`,
		`<script type="module">import "/js/app.js"</script>`,
		`<nav class="panel-tabs tabbed-pane-header" aria-label="GoLazy panel sections">`,
		`<span class="panel-tab">App Logs</span>`,
		`<a href="/_golazy/requests" data-turbo-frame="_top">Requests</a>`,
		`data-panel-close`,
		`<section id="logs" class="tool-view is-active" data-view="logs">`,
		`data-panel-resize-direction-value="right"`,
		`class="runtime-left-stack" data-panel-resize-target="primary"`,
		`<section class="runtime-pane output-pane">`,
		`<turbo-frame id="status_bar" src="/_golazy/status" data-turbo-permanent></turbo-frame>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `<div class="panel-title">GoLazy</div>`) {
		t.Fatalf("body renders obsolete layout title:\n%s", body)
	}
	if strings.Contains(body, `src="/_golazy/requests"`) {
		t.Fatalf("body renders inactive tab frame src:\n%s", body)
	}
}

func TestPanelFrameRoutes(t *testing.T) {
	app := testApp()

	for _, test := range []struct {
		path  string
		frame string
		want  string
	}{
		{path: "/_golazy/console", frame: "console", want: `data-view="console"`},
		{path: "/_golazy/logs", frame: "logs", want: `data-view="logs"`},
		{path: "/_golazy/services", frame: "services", want: `data-view="services"`},
		{path: "/_golazy/traces", frame: "traces", want: `data-traces-panel`},
		{path: "/_golazy/status", frame: "status_bar", want: `<a class="app-status-chip" href="/_golazy/logs" data-turbo-frame="_top" data-app-status="running">`},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.Header.Set("Turbo-Frame", test.frame)
		response := httptest.NewRecorder()
		app.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d: %s", test.path, response.Code, http.StatusOK, response.Body.String())
		}
		body := response.Body.String()
		if !strings.Contains(body, `<turbo-frame id="`+test.frame+`">`) {
			t.Fatalf("%s body missing frame %q:\n%s", test.path, test.frame, body)
		}
		if !strings.Contains(body, test.want) {
			t.Fatalf("%s body missing %q:\n%s", test.path, test.want, body)
		}
		if test.path == "/_golazy/status" && !strings.Contains(body, `class="service-status-button" data-service-status data-service-name="postgres" data-service-state="ready"`) {
			t.Fatalf("%s body missing service status button:\n%s", test.path, body)
		}
	}
}

func TestPanelAssetsAndJobsJSON(t *testing.T) {
	app := testApp()

	asset := httptest.NewRecorder()
	app.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/_golazy/assets/importmap.json", nil))
	if asset.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want %d: %s", asset.Code, http.StatusOK, asset.Body.String())
	}
	if !strings.Contains(asset.Body.String(), `"/js/app.js"`) {
		t.Fatalf("importmap body = %s, want /js/app.js", asset.Body.String())
	}

	panelScript := httptest.NewRecorder()
	app.ServeHTTP(panelScript, httptest.NewRequest(http.MethodGet, "/_golazy/assets/panel.js", nil))
	if panelScript.Code != http.StatusOK {
		t.Fatalf("panel script status = %d, want %d: %s", panelScript.Code, http.StatusOK, panelScript.Body.String())
	}
	for _, want := range []string{
		"window.__golazyDevPanelClient",
		"window.__lazyReloadSource",
		"EventSource.CLOSED",
		"Resize GoLazy development panel",
		"golazy:devpanel:height",
		`location.pathname.startsWith("/_golazy")`,
	} {
		if !strings.Contains(panelScript.Body.String(), want) {
			t.Fatalf("panel script missing %q:\n%s", want, panelScript.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/_golazy/jobs", nil)
	request.Header.Set("Accept", "application/json")
	jobs := httptest.NewRecorder()
	app.ServeHTTP(jobs, request)
	if jobs.Code != http.StatusServiceUnavailable {
		t.Fatalf("jobs status = %d, want %d", jobs.Code, http.StatusServiceUnavailable)
	}
}

func TestPanelEventsStreamThroughAppMiddleware(t *testing.T) {
	app := testApp()
	server := httptest.NewServer(app)
	defer server.Close()

	response, err := server.Client().Get(server.URL + "/_golazy/events")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got, want := response.Header.Get("Content-Type"), "text/event-stream"; got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}

	reader := bufio.NewReader(response.Body)
	var lines []string
	for len(lines) < 3 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	if got, want := strings.Join(lines, ""), "event: ready\ndata: ok\n\n"; got != want {
		t.Fatalf("first event = %q, want %q", got, want)
	}
}

func testApp() http.Handler {
	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:       buildservice.StateRunning,
		Message:     "running",
		BuildCount:  3,
		AppAddr:     "127.0.0.1:3001",
		WatchedRoot: ".",
	})
	store.UpdateService("postgres", buildservice.ServiceReady, "ready")
	return App(Config{
		Store:             store,
		Actions:           buildservice.NewActions(),
		ForceDetailErrors: true,
	})
}
