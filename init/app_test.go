package appinit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golazy.dev/lazy/services/buildservice"
)

func TestPanelShellLoadsImportmapAndFrames(t *testing.T) {
	app := testApp()

	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{
		`<script type="importmap">`,
		`"/js/app.js": "/_golazy/assets/lazyshaft/app/`,
		`<script type="module">import "/js/app.js"</script>`,
		`<turbo-frame id="logs" src="/_golazy/logs"></turbo-frame>`,
		`<turbo-frame id="status_bar" src="/_golazy/status"></turbo-frame>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
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
		{path: "/_golazy/status", frame: "status_bar", want: `class="status-bar"`},
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

	request := httptest.NewRequest(http.MethodGet, "/_golazy/jobs", nil)
	request.Header.Set("Accept", "application/json")
	jobs := httptest.NewRecorder()
	app.ServeHTTP(jobs, request)
	if jobs.Code != http.StatusServiceUnavailable {
		t.Fatalf("jobs status = %d, want %d", jobs.Code, http.StatusServiceUnavailable)
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
	return App(Config{
		Store:             store,
		Actions:           buildservice.NewActions(),
		ForceDetailErrors: true,
	})
}
