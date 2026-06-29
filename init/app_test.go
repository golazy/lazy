package appinit

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"golazy.dev/lazy/services/buildservice"
)

func TestPanelRootRedirectsToApp(t *testing.T) {
	app := testApp()

	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy", nil))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusSeeOther, response.Body.String())
	}
	if got, want := response.Header().Get("Location"), "/_golazy/app"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestDevToolsWorkspaceRoutePointsAtAppJS(t *testing.T) {
	root := t.TempDir()
	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:       buildservice.StateRunning,
		Message:     "running",
		WatchedRoot: root,
	})
	app := App(Config{
		Store:             store,
		Actions:           buildservice.NewActions(),
		ForceDetailErrors: true,
	})

	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/.well-known/appspecific/com.chrome.devtools.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json; charset=utf-8", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	var got struct {
		Workspace struct {
			Root string `json:"root"`
			UUID string `json:"uuid"`
		} `json:"workspace"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v\n%s", err, response.Body.String())
	}
	if want := filepath.Join(root, "app", "js"); got.Workspace.Root != want {
		t.Fatalf("workspace.root = %q, want %q", got.Workspace.Root, want)
	}
	if !validDevToolsWorkspaceUUID(got.Workspace.UUID) {
		t.Fatalf("workspace.uuid = %q, want valid UUID", got.Workspace.UUID)
	}

	again := httptest.NewRecorder()
	app.ServeHTTP(again, httptest.NewRequest(http.MethodGet, "/.well-known/appspecific/com.chrome.devtools.json", nil))
	var gotAgain struct {
		Workspace struct {
			UUID string `json:"uuid"`
		} `json:"workspace"`
	}
	if err := json.Unmarshal(again.Body.Bytes(), &gotAgain); err != nil {
		t.Fatalf("decode second response: %v\n%s", err, again.Body.String())
	}
	if gotAgain.Workspace.UUID != got.Workspace.UUID {
		t.Fatalf("workspace.uuid changed between requests: %q then %q", got.Workspace.UUID, gotAgain.Workspace.UUID)
	}
}

func TestPanelTabPageLoadsImportmapNavAndPermanentStatus(t *testing.T) {
	app := testApp()

	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy/app", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{
		`<script type="importmap">`,
		`"app.js": "/_golazy/assets/lazyshaft/app/`,
		`<script type="module">import "app.js"</script>`,
		`<nav class="panel-tabs tabbed-pane-header" aria-label="GoLazy panel sections" data-controller="panel-close">`,
		`<span class="panel-tab">App</span>`,
		`<a href="/_golazy/requests" data-turbo-frame="_top">Requests</a>`,
		`<a href="/_golazy/buildinfo" data-turbo-frame="_top">BuildInfo</a>`,
		`<a href="/_golazy/dependencies" data-turbo-frame="_top">Dependencies</a>`,
		`data-panel-close`,
		`<turbo-stream-source src="/_golazy/app"></turbo-stream-source>`,
		`<section id="app" class="tool-view is-active app-view" data-view="app" data-app-panel>`,
		`<table class="data-grid app-log-grid" data-controller="table-resize">`,
		`<button type="submit" class="toolbar-button">Rebuild</button>`,
		`<button type="submit" class="toolbar-button">Restart</button>`,
		`<a class="toolbar-button" href="http://127.0.0.1:3001" target="_blank" rel="noreferrer">Open App</a>`,
		`<turbo-frame id="status_bar" src="/_golazy/status" data-turbo-permanent></turbo-frame>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `<div class="panel-title">GoLazy</div>`) {
		t.Fatalf("body renders obsolete layout title:\n%s", body)
	}
	if strings.Contains(body, `/_golazy/console`) || strings.Contains(body, `>Console<`) {
		t.Fatalf("body renders removed Console tab:\n%s", body)
	}
	if strings.Contains(body, `>App Logs<`) || strings.Contains(body, `/_golazy/logs`) {
		t.Fatalf("body renders removed App Logs tab:\n%s", body)
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
		{path: "/_golazy/app", frame: "app", want: `data-view="app"`},
		{path: "/_golazy/services", frame: "services", want: `data-view="services"`},
		{path: "/_golazy/traces", frame: "traces", want: `data-traces-panel`},
		{path: "/_golazy/dependencies", frame: "dependencies", want: `data-dependencies-panel`},
		{path: "/_golazy/status", frame: "status_bar", want: `<a class="app-status-chip" href="/_golazy/app" data-turbo-frame="_top" data-app-status="running">`},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.Header.Set("Turbo-Frame", test.frame)
		response := httptest.NewRecorder()
		app.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d: %s", test.path, response.Code, http.StatusOK, response.Body.String())
		}
		body := response.Body.String()
		frameTag := `<turbo-frame id="` + test.frame + `">`
		if test.path == "/_golazy/status" {
			frameTag = `<turbo-frame id="status_bar" data-turbo-permanent>`
		}
		if !strings.Contains(body, frameTag) {
			t.Fatalf("%s body missing frame %q:\n%s", test.path, test.frame, body)
		}
		if !strings.Contains(body, test.want) {
			t.Fatalf("%s body missing %q:\n%s", test.path, test.want, body)
		}
		if test.path == "/_golazy/status" && !strings.Contains(body, `<turbo-stream-source src="/_golazy/status"></turbo-stream-source>`) {
			t.Fatalf("%s body missing status stream source:\n%s", test.path, body)
		}
		if test.path == "/_golazy/status" && !strings.Contains(body, `<a class="service-status-button" href="/_golazy/services?service=postgres" data-turbo-frame="_top" data-service-state="ready" title="ready">`) {
			t.Fatalf("%s body missing service status button:\n%s", test.path, body)
		}
	}
}

func TestPanelRoutesPageFetchesApplicationRoutes(t *testing.T) {
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`[
			{"method":"GET","path":"/posts","name":"posts","controller":"posts","action":"Index"},
			{"method":"GET","path":"/posts/{post_id}","name":"post","controller":"posts","action":"Show","params":{"post_id":true}},
			{"method":"GET","path":"/admin","name":"admin","controller":"admin","action":"Index"}
		]`))
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		Message:          "running",
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	app := App(Config{
		Store:             store,
		Actions:           buildservice.NewActions(),
		ForceDetailErrors: true,
	})

	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy/routes?q=post_id", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if gotPath != "/routes" {
		t.Fatalf("app control path = %q, want /routes", gotPath)
	}
	body := response.Body.String()
	for _, want := range []string{
		`<table class="data-grid routes-grid" data-controller="table-resize">`,
		`<tbody data-routes-list>`,
		`1 / 3 routes`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `<code>/posts/{post_id}</code>`) || strings.Contains(body, `posts#Show`) {
		t.Fatalf("body rendered route rows before stream hydration:\n%s", body)
	}
	if strings.Contains(body, `<code>/admin</code>`) {
		t.Fatalf("body includes unfiltered route:\n%s", body)
	}
}

func TestPanelRoutesStreamHydratesApplicationRoutes(t *testing.T) {
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`[
			{"method":"GET","path":"/posts","name":"posts","controller":"posts","action":"Index"},
			{"method":"GET","path":"/posts/{post_id}","name":"post","controller":"posts","action":"Show","params":{"post_id":true}},
			{"method":"GET","path":"/admin","name":"admin","controller":"admin","action":"Index"}
		]`))
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		Message:          "running",
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	app := App(Config{
		Store:             store,
		Actions:           buildservice.NewActions(),
		ForceDetailErrors: true,
	})
	server := httptest.NewServer(app)
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/_golazy/routes?q=post_id", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Accept", "text/event-stream")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	payload := readSSEPayload(t, bufio.NewReader(response.Body))
	if gotPath != "/routes" {
		t.Fatalf("app control path = %q, want /routes", gotPath)
	}
	for _, want := range []string{
		`data: <turbo-stream action="update" targets="[data-routes-list]">`,
		`<code>/posts/{post_id}</code>`,
		`posts#Show`,
		`<turbo-stream action="update" targets="[data-routes-count]">`,
		`1 / 3 routes`,
	} {
		if !strings.Contains(payload, want) {
			t.Fatalf("stream payload missing %q:\n%s", want, payload)
		}
	}
	if strings.Contains(payload, `action="replace" target="routes"`) {
		t.Fatalf("stream replaced whole routes frame:\n%s", payload)
	}
}

func TestPanelDependenciesPageFetchesApplicationGraph(t *testing.T) {
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{
			"nodes":["app","db","posts"],
			"edges":[
				{"from":"app","to":"db"},
				{"from":"app","to":"posts"},
				{"from":"posts","to":"db"}
			]
		}`))
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		Message:          "running",
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	app := App(Config{
		Store:             store,
		Actions:           buildservice.NewActions(),
		ForceDetailErrors: true,
	})

	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy/dependencies", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if gotPath != "/dependencies" {
		t.Fatalf("app control path = %q, want /dependencies", gotPath)
	}
	body := response.Body.String()
	for _, want := range []string{
		`<turbo-stream-source src="/_golazy/dependencies"></turbo-stream-source>`,
		`<section id="dependencies" class="tool-view is-active dependencies-view" data-view="dependencies" data-dependencies-panel>`,
		`<div class="dependencies-layout" data-controller="depgraph">`,
		`data-depgraph-name="posts"`,
		`data-controller-depgraph-depends-on="db"`,
		`2 services`,
		`<td><code>posts</code></td>`,
		`<td><code>db</code></td>`,
		`<td><code>app</code></td>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func readSSEPayload(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	payload := make(chan string, 1)
	errs := make(chan error, 1)
	go func() {
		var builder strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				errs <- err
				return
			}
			builder.WriteString(line)
			if line == "\n" {
				payload <- builder.String()
				return
			}
		}
	}()
	select {
	case got := <-payload:
		return got
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for turbo-stream")
	}
	return ""
}

func TestPanelAssetsAndJobsPage(t *testing.T) {
	app := testApp()

	asset := httptest.NewRecorder()
	app.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/_golazy/assets/importmap.json", nil))
	if asset.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want %d: %s", asset.Code, http.StatusOK, asset.Body.String())
	}
	if !strings.Contains(asset.Body.String(), `"app.js"`) {
		t.Fatalf("importmap body = %s, want app.js", asset.Body.String())
	}

	panelScript := httptest.NewRecorder()
	app.ServeHTTP(panelScript, httptest.NewRequest(http.MethodGet, "/_golazy/assets/panel.js", nil))
	if panelScript.Code != http.StatusOK {
		t.Fatalf("panel script status = %d, want %d: %s", panelScript.Code, http.StatusOK, panelScript.Body.String())
	}
	for _, want := range []string{
		"window.__golazyDevPanelClient",
		"window.disableDevPanel",
		"window.__lazyReloadSource",
		"/_golazy/assets/devpanel_controller.js",
		"EventSource.CLOSED",
		`location.pathname.startsWith("/_golazy")`,
	} {
		if !strings.Contains(panelScript.Body.String(), want) {
			t.Fatalf("panel script missing %q:\n%s", want, panelScript.Body.String())
		}
	}

	controllerScript := httptest.NewRecorder()
	app.ServeHTTP(controllerScript, httptest.NewRequest(http.MethodGet, "/_golazy/assets/devpanel_controller.js", nil))
	if controllerScript.Code != http.StatusOK {
		t.Fatalf("controller script status = %d, want %d: %s", controllerScript.Code, http.StatusOK, controllerScript.Body.String())
	}
	for _, want := range []string{
		"DevPanelController",
		"golazy:page:devpanel-ready",
		"golazy:devpanel:height",
		"installLauncher",
		"golazyDevPanelLauncherBound",
		"shouldShowLauncher",
		"togglePanel",
	} {
		if !strings.Contains(controllerScript.Body.String(), want) {
			t.Fatalf("controller script missing %q:\n%s", want, controllerScript.Body.String())
		}
	}

	logo := httptest.NewRecorder()
	app.ServeHTTP(logo, httptest.NewRequest(http.MethodGet, "/_golazy/assets/logo-square.svg", nil))
	if logo.Code != http.StatusOK {
		t.Fatalf("logo status = %d, want %d: %s", logo.Code, http.StatusOK, logo.Body.String())
	}
	for _, want := range []string{
		`<rect width="1000" height="1000" fill="#FBBC04"/>`,
		`viewBox="0 0 1000 1000"`,
	} {
		if !strings.Contains(logo.Body.String(), want) {
			t.Fatalf("logo missing %q:\n%s", want, logo.Body.String())
		}
	}

	jobs := httptest.NewRecorder()
	app.ServeHTTP(jobs, httptest.NewRequest(http.MethodGet, "/_golazy/jobs", nil))
	if jobs.Code != http.StatusOK {
		t.Fatalf("jobs status = %d, want %d: %s", jobs.Code, http.StatusOK, jobs.Body.String())
	}
	for _, want := range []string{
		`<turbo-stream-source src="/_golazy/jobs"></turbo-stream-source>`,
		`<section id="jobs" class="tool-view is-active" data-view="jobs">`,
		`Jobs unavailable`,
	} {
		if !strings.Contains(jobs.Body.String(), want) {
			t.Fatalf("jobs body missing %q:\n%s", want, jobs.Body.String())
		}
	}

	buildInfo := httptest.NewRecorder()
	app.ServeHTTP(buildInfo, httptest.NewRequest(http.MethodGet, "/_golazy/buildinfo", nil))
	if buildInfo.Code != http.StatusOK {
		t.Fatalf("buildinfo status = %d, want %d: %s", buildInfo.Code, http.StatusOK, buildInfo.Body.String())
	}
	for _, want := range []string{
		`<turbo-stream-source src="/_golazy/buildinfo"></turbo-stream-source>`,
		`<section id="buildinfo" class="tool-view is-active buildinfo-view" data-view="buildinfo" data-buildinfo-panel>`,
		`BuildInfo unavailable`,
	} {
		if !strings.Contains(buildInfo.Body.String(), want) {
			t.Fatalf("buildinfo body missing %q:\n%s", want, buildInfo.Body.String())
		}
	}

	assets := httptest.NewRecorder()
	app.ServeHTTP(assets, httptest.NewRequest(http.MethodGet, "/_golazy/assets", nil))
	if assets.Code != http.StatusOK {
		t.Fatalf("assets status = %d, want %d: %s", assets.Code, http.StatusOK, assets.Body.String())
	}
	for _, want := range []string{
		`<turbo-stream-source src="/_golazy/assets"></turbo-stream-source>`,
		`<section id="assets" class="tool-view is-active" data-view="assets">`,
		`<table class="data-grid assets-grid" data-controller="table-resize">`,
	} {
		if !strings.Contains(assets.Body.String(), want) {
			t.Fatalf("assets body missing %q:\n%s", want, assets.Body.String())
		}
	}
}

func TestPanelStatusStreamThroughAppMiddleware(t *testing.T) {
	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:      buildservice.StateRunning,
		Message:    "running",
		BuildCount: 3,
		AppAddr:    "127.0.0.1:3001",
	})
	app := App(Config{
		Store:             store,
		Actions:           buildservice.NewActions(),
		ForceDetailErrors: true,
	})
	server := httptest.NewServer(app)
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/_golazy/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Accept", "text/event-stream")
	response, err := server.Client().Do(request)
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
	payload := make(chan string, 1)
	errs := make(chan error, 1)
	go func() {
		var builder strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				errs <- err
				return
			}
			builder.WriteString(line)
			if line == "\n" {
				payload <- builder.String()
				return
			}
		}
	}()

	store.Update(buildservice.Snapshot{
		State:      buildservice.StateRunning,
		Message:    "updated",
		BuildCount: 4,
		AppAddr:    "127.0.0.1:3001",
	})

	select {
	case got := <-payload:
		for _, want := range []string{
			`data: <turbo-stream action="replace" target="status_bar_content">`,
			`<footer id="status_bar_content" class="status-bar">`,
			`Build <span>4</span>`,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("stream event missing %q:\n%s", want, got)
			}
		}
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for status turbo-stream")
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

func validDevToolsWorkspaceUUID(value string) bool {
	return regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(value)
}
