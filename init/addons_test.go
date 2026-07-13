package appinit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontrolplane"
)

func TestAddonsPanelRendersEscapedDeterministicDescriptors(t *testing.T) {
	panels := []lazycontrolplane.PanelInfo{
		{
			ID:          "zeta",
			Owner:       "example.test/zeta",
			Title:       "Zeta",
			Description: "Second panel",
			EndpointID:  "zeta.panel",
			Pattern:     "GET /addons/zeta",
			Method:      http.MethodGet,
			Path:        "/addons/zeta",
			Order:       20,
		},
		{
			ID:          "alpha",
			Owner:       `example.test/<owner>`,
			Title:       `<script>alert("title")</script>`,
			Description: `<img src=x onerror=alert("description")>`,
			EndpointID:  "alpha.panel",
			Pattern:     "GET /addons/alpha",
			Method:      http.MethodGet,
			Path:        "/addons/alpha",
			Actions: []lazycontrolplane.PanelActionInfo{{
				ID:          "refresh",
				Title:       `<b>Refresh</b>`,
				Description: `Run "refresh" now`,
				EndpointID:  "alpha.refresh",
				Pattern:     "POST /addons/alpha/refresh",
				Method:      http.MethodPost,
				Path:        "/addons/alpha/refresh",
			}},
			Order: 10,
		},
	}
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != lazycontrolplane.PanelsPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(lazycontrolplane.PanelsResponse{Schema: 1, Panels: panels})
	}))
	defer appControl.Close()

	app := addonPanelTestApp(appControl)
	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy/addons", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{
		`<span class="panel-tab">Add-ons</span>`,
		`<section id="addons" class="tool-view is-active" data-view="addons">`,
		`&lt;script&gt;alert(&#34;title&#34;)&lt;/script&gt;`,
		`&lt;img src=x onerror=alert(&#34;description&#34;)&gt;`,
		`example.test/&lt;owner&gt;`,
		`href="/_golazy/addons?panel=alpha"`,
		`src="/_golazy/addons/content?panel=alpha"`,
		`name="addon-panel-frame"`,
		`title="&lt;script&gt;alert(&#34;title&#34;)&lt;/script&gt;" sandbox="" referrerpolicy="no-referrer"`,
		`action="/_golazy/addons/action" target="addon-panel-frame"`,
		`name="panel" value="alpha"`,
		`name="action" value="refresh"`,
		`&lt;b&gt;Refresh&lt;/b&gt;`,
		`title="Run &#34;refresh&#34; now"`,
		`<code>/addons/alpha</code>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `<script>alert("title")</script>`) || strings.Contains(body, `<img src=x onerror=`) {
		t.Fatalf("descriptor markup was not escaped:\n%s", body)
	}
	if alpha, zeta := strings.Index(body, `href="/_golazy/addons?panel=alpha"`), strings.Index(body, `href="/_golazy/addons?panel=zeta"`); alpha < 0 || zeta < 0 || alpha >= zeta {
		t.Fatalf("descriptors are not in deterministic order:\n%s", body)
	}
}

func TestAddonsActionRouteUsesRenderedCSRFToken(t *testing.T) {
	actionRequests := 0
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case lazycontrolplane.PanelsPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"schema":1,"panels":[{"id":"alpha","owner":"example.test/addon","title":"Alpha","description":"Status","endpoint_id":"alpha.panel","pattern":"GET /addons/alpha","method":"GET","path":"/addons/alpha","actions":[{"id":"refresh","title":"Refresh","description":"Refresh status","endpoint_id":"alpha.refresh","pattern":"POST /addons/alpha/refresh","method":"POST","path":"/addons/alpha/refresh"}],"order":0}]}`)
		case "/addons/alpha/refresh":
			actionRequests++
			if r.Method != http.MethodPost || r.ContentLength > 0 || r.URL.RawQuery != "" {
				t.Errorf("upstream action request = %s %s with length %d", r.Method, r.URL.String(), r.ContentLength)
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprint(w, "refreshed")
		default:
			http.NotFound(w, r)
		}
	}))
	defer appControl.Close()
	app := addonPanelTestApp(appControl)

	index := httptest.NewRecorder()
	app.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/_golazy/addons", nil))
	match := regexp.MustCompile(`name="csrf_token" value="([A-Za-z0-9_-]+)"`).FindStringSubmatch(index.Body.String())
	if len(match) != 2 {
		t.Fatalf("rendered page is missing the action CSRF token:\n%s", index.Body.String())
	}
	form := url.Values{"panel": {"alpha"}, "action": {"refresh"}, "csrf_token": {match[1]}}
	request := httptest.NewRequest(http.MethodPost, "/_golazy/addons/action", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.com")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "refreshed" {
		t.Fatalf("action response = %d %q", response.Code, response.Body.String())
	}
	if actionRequests != 1 {
		t.Fatalf("action endpoint requests = %d, want one", actionRequests)
	}
}

func TestAddonsPanelRendersEmptyAndUnavailableStates(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"schema":1,"panels":[]}`)
		}))
		defer appControl.Close()

		response := httptest.NewRecorder()
		addonPanelTestApp(appControl).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy/addons", nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "No add-on panels") {
			t.Fatalf("empty response = %d %q", response.Code, response.Body.String())
		}
	})

	t.Run("unavailable", func(t *testing.T) {
		response := httptest.NewRecorder()
		testApp().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy/addons", nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Panels unavailable") {
			t.Fatalf("unavailable response = %d %q", response.Code, response.Body.String())
		}
	})
}

func TestAddonsContentRouteAllowsOnlyGetAndHead(t *testing.T) {
	panelRequests := 0
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case lazycontrolplane.PanelsPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"schema":1,"panels":[{"id":"alpha","owner":"example.test/addon","title":"Alpha","description":"Status","endpoint_id":"alpha.panel","pattern":"GET /addons/alpha","method":"GET","path":"/addons/alpha","order":0}]}`)
		case "/addons/alpha":
			panelRequests++
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprint(w, "ready")
		default:
			http.NotFound(w, r)
		}
	}))
	defer appControl.Close()
	app := addonPanelTestApp(appControl)

	get := httptest.NewRecorder()
	app.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/_golazy/addons/content?panel=alpha", nil))
	if get.Code != http.StatusOK || get.Body.String() != "ready" {
		t.Fatalf("GET response = %d %q", get.Code, get.Body.String())
	}

	head := httptest.NewRecorder()
	app.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/_golazy/addons/content?panel=alpha", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d %q", head.Code, head.Body.String())
	}

	post := httptest.NewRecorder()
	app.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/_golazy/addons/content?panel=alpha", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want %d", post.Code, http.StatusMethodNotAllowed)
	}
	if got := post.Header().Get("Allow"); !strings.Contains(got, http.MethodGet) || !strings.Contains(got, http.MethodHead) {
		t.Fatalf("POST Allow = %q, want GET and HEAD", got)
	}
	if panelRequests != 2 {
		t.Fatalf("panel endpoint requests = %d, want GET and HEAD only", panelRequests)
	}
}

func addonPanelTestApp(appControl *httptest.Server) http.Handler {
	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		Message:          "running",
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	return App(Config{
		Store:             store,
		Actions:           buildservice.NewActions(),
		ForceDetailErrors: true,
	})
}
