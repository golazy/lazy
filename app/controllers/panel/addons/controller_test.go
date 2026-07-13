package addons

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontrolplane"
)

func TestContentResolvesDescriptorAndSanitizesResponse(t *testing.T) {
	var mu sync.Mutex
	var endpointMethod string
	var endpointPath string
	var endpointQuery string
	var endpointCookie string
	appControl := panelTestServer(t, []lazycontrolplane.PanelInfo{testPanel("status", "/addons/status")}, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		endpointMethod = r.Method
		endpointPath = r.URL.Path
		endpointQuery = r.URL.RawQuery
		endpointCookie = r.Header.Get("Cookie")
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Set-Cookie", "session=stolen")
		w.Header().Set("Location", "https://attacker.example/")
		w.Header().Set("Refresh", "0; url=https://attacker.example/")
		w.Header().Set("Content-Security-Policy", "default-src * 'unsafe-inline' 'unsafe-eval'")
		w.Header().Set("X-Upstream-Secret", "do-not-copy")
		_, _ = fmt.Fprint(w, `<h1>PostgreSQL</h1><script>window.top.location="https://attacker.example"</script>`)
	})
	defer appControl.Close()

	controller := testAddonsController(appControl)
	request := httptest.NewRequest(http.MethodGet, "/_golazy/addons/content?panel=status&path=/admin&url=https://attacker.example", nil)
	request.Header.Set("Cookie", "panel=session")
	response := httptest.NewRecorder()
	if err := controller.Content(response, request); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	mu.Lock()
	gotMethod, gotPath, gotQuery, gotCookie := endpointMethod, endpointPath, endpointQuery, endpointCookie
	mu.Unlock()
	if gotMethod != http.MethodGet || gotPath != "/addons/status" || gotQuery != "" {
		t.Fatalf("proxied request = %s %s?%s, want GET /addons/status with no query", gotMethod, gotPath, gotQuery)
	}
	if gotCookie != "" {
		t.Fatalf("proxied Cookie = %q, want empty", gotCookie)
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	for _, name := range []string{"Set-Cookie", "Location", "Refresh", "X-Upstream-Secret"} {
		if got := response.Header().Get(name); got != "" {
			t.Fatalf("%s = %q, want stripped", name, got)
		}
	}
	if got := response.Header().Get("Content-Security-Policy"); got != panelContentSecurityPolicy {
		t.Fatalf("Content-Security-Policy = %q, want host policy", got)
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if !strings.Contains(response.Body.String(), "PostgreSQL") {
		t.Fatalf("body = %q, want panel content", response.Body.String())
	}
}

func TestContentSupportsHeadWithoutReturningBody(t *testing.T) {
	var gotMethod string
	appControl := panelTestServer(t, []lazycontrolplane.PanelInfo{testPanel("status", "/addons/status")}, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ready":true}`)
	})
	defer appControl.Close()

	controller := testAddonsController(appControl)
	response := httptest.NewRecorder()
	if err := controller.Content(response, httptest.NewRequest(http.MethodHead, "/_golazy/addons/content?panel=status", nil)); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d %q, want 200 with empty body", response.Code, response.Body.String())
	}
	if gotMethod != http.MethodHead {
		t.Fatalf("upstream method = %q, want HEAD", gotMethod)
	}
}

func TestContentRejectsUnsafeDescriptorPathBeforeProxying(t *testing.T) {
	requestedPanel := false
	descriptor := testPanel("unsafe", "https://attacker.example/panel")
	descriptor.Pattern = "GET https://attacker.example/panel"
	appControl := panelTestServer(t, []lazycontrolplane.PanelInfo{descriptor}, func(http.ResponseWriter, *http.Request) {
		requestedPanel = true
	})
	defer appControl.Close()

	controller := testAddonsController(appControl)
	response := httptest.NewRecorder()
	if err := controller.Content(response, httptest.NewRequest(http.MethodGet, "/_golazy/addons/content?panel=unsafe", nil)); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
	if requestedPanel {
		t.Fatal("unsafe descriptor caused a panel endpoint request")
	}
}

func TestContentRejectsOversizeAndUnsupportedResponses(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		status      int
		location    string
	}{
		{name: "oversize", contentType: "text/plain", body: strings.Repeat("x", maxPanelBodyBytes+1), status: http.StatusOK},
		{name: "binary", contentType: "application/octet-stream", body: "binary", status: http.StatusOK},
		{name: "redirect", contentType: "text/html", status: http.StatusFound, location: "https://attacker.example/"},
		{name: "non utf8", contentType: "text/plain", body: string([]byte{0xff, 0xfe}), status: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appControl := panelTestServer(t, []lazycontrolplane.PanelInfo{testPanel("status", "/addons/status")}, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				if test.location != "" {
					w.Header().Set("Location", test.location)
				}
				w.WriteHeader(test.status)
				_, _ = fmt.Fprint(w, test.body)
			})
			defer appControl.Close()

			controller := testAddonsController(appControl)
			response := httptest.NewRecorder()
			if err := controller.Content(response, httptest.NewRequest(http.MethodGet, "/_golazy/addons/content?panel=status", nil)); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
			}
			if got := response.Header().Get("Location"); got != "" {
				t.Fatalf("Location = %q, want stripped", got)
			}
			if strings.Contains(response.Body.String(), test.body) && test.body != "" {
				t.Fatal("rejected upstream body leaked into the response")
			}
		})
	}
}

func TestContentTimesOutSlowEndpoint(t *testing.T) {
	appControl := panelTestServer(t, []lazycontrolplane.PanelInfo{testPanel("slow", "/addons/slow")}, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "late")
	})
	defer appControl.Close()

	controller := testAddonsController(appControl)
	controller.timeout = 10 * time.Millisecond
	response := httptest.NewRecorder()
	if err := controller.Content(response, httptest.NewRequest(http.MethodGet, "/_golazy/addons/content?panel=slow", nil)); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
}

func TestActionResolvesDescriptorAndSanitizesResponse(t *testing.T) {
	panelDescriptor := testPanel("status", "/addons/status")
	panelDescriptor.Actions = []lazycontrolplane.PanelActionInfo{testAction("refresh", "/addons/status/refresh")}
	var mu sync.Mutex
	var endpointMethod, endpointPath, endpointQuery, endpointCookie, endpointOrigin, endpointBody string
	appControl := panelTestServer(t, []lazycontrolplane.PanelInfo{panelDescriptor}, func(w http.ResponseWriter, r *http.Request) {
		body, _ := readBounded(r.Body, maxActionBodyBytes)
		mu.Lock()
		endpointMethod = r.Method
		endpointPath = r.URL.Path
		endpointQuery = r.URL.RawQuery
		endpointCookie = r.Header.Get("Cookie")
		endpointOrigin = r.Header.Get("Origin")
		endpointBody = string(body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Set-Cookie", "control=secret")
		w.Header().Set("Location", "https://attacker.example/")
		_, _ = fmt.Fprint(w, `{"refreshed":true}`)
	})
	defer appControl.Close()

	controller := testAddonsController(appControl)
	request := validActionRequest("status", "refresh")
	request.Header.Set("Cookie", "panel=session")
	request.Header.Set("Origin", "http://example.com")
	request.Header.Set("Referer", "http://example.com/_golazy/addons?panel=status")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	response := httptest.NewRecorder()
	if err := controller.Action(response, request); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	mu.Lock()
	gotMethod, gotPath, gotQuery := endpointMethod, endpointPath, endpointQuery
	gotCookie, gotOrigin, gotBody := endpointCookie, endpointOrigin, endpointBody
	mu.Unlock()
	if gotMethod != http.MethodPost || gotPath != "/addons/status/refresh" || gotQuery != "" {
		t.Fatalf("proxied request = %s %s?%s, want empty POST to descriptor path", gotMethod, gotPath, gotQuery)
	}
	if gotCookie != "" || gotOrigin != "" || gotBody != "" {
		t.Fatalf("proxied credentials/body = cookie %q origin %q body %q, want empty", gotCookie, gotOrigin, gotBody)
	}
	if got := response.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("Set-Cookie = %q, want stripped", got)
	}
	if got := response.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want stripped", got)
	}
	if got := response.Header().Get("Content-Security-Policy"); got != panelContentSecurityPolicy {
		t.Fatalf("Content-Security-Policy = %q, want host policy", got)
	}
	if got, want := response.Body.String(), `{"refreshed":true}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestActionRejectsUntrustedBrowserInput(t *testing.T) {
	panelDescriptor := testPanel("status", "/addons/status")
	panelDescriptor.Actions = []lazycontrolplane.PanelActionInfo{testAction("refresh", "/addons/status/refresh")}
	requests := 0
	appControl := panelTestServer(t, []lazycontrolplane.PanelInfo{panelDescriptor}, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "refreshed")
	})
	defer appControl.Close()
	controller := testAddonsController(appControl)

	tests := []struct {
		name   string
		change func(*http.Request)
		status int
	}{
		{name: "method", change: func(request *http.Request) { request.Method = http.MethodGet }, status: http.StatusMethodNotAllowed},
		{name: "query", change: func(request *http.Request) { request.URL.RawQuery = "url=https://attacker.example" }, status: http.StatusBadRequest},
		{name: "content type", change: func(request *http.Request) { request.Header.Set("Content-Type", "application/json") }, status: http.StatusUnsupportedMediaType},
		{name: "fetch site", change: func(request *http.Request) { request.Header.Set("Sec-Fetch-Site", "cross-site") }, status: http.StatusForbidden},
		{name: "origin", change: func(request *http.Request) { request.Header.Set("Origin", "https://attacker.example") }, status: http.StatusForbidden},
		{name: "referrer", change: func(request *http.Request) { request.Header.Set("Referer", "https://attacker.example/panel") }, status: http.StatusForbidden},
		{name: "csrf", change: func(request *http.Request) {
			replaceActionForm(request, url.Values{"panel": {"status"}, "action": {"refresh"}, "csrf_token": {"invalid"}})
		}, status: http.StatusForbidden},
		{name: "extra field", change: func(request *http.Request) {
			replaceActionForm(request, url.Values{"panel": {"status"}, "action": {"refresh"}, "csrf_token": {actionCSRFToken}, "url": {"https://attacker.example"}})
		}, status: http.StatusBadRequest},
		{name: "duplicate ID", change: func(request *http.Request) {
			replaceActionForm(request, url.Values{"panel": {"status", "other"}, "action": {"refresh"}, "csrf_token": {actionCSRFToken}})
		}, status: http.StatusBadRequest},
		{name: "oversize", change: func(request *http.Request) {
			replaceActionForm(request, url.Values{"panel": {strings.Repeat("x", maxActionBodyBytes)}, "action": {"refresh"}, "csrf_token": {actionCSRFToken}})
		}, status: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validActionRequest("status", "refresh")
			test.change(request)
			response := httptest.NewRecorder()
			if err := controller.Action(response, request); err != nil {
				t.Fatal(err)
			}
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.status, response.Body.String())
			}
		})
	}
	if requests != 0 {
		t.Fatalf("untrusted requests invoked %d action endpoints, want zero", requests)
	}
}

func TestActionResolvesOnlyDeclaredPanelActionIDs(t *testing.T) {
	panelDescriptor := testPanel("status", "/addons/status")
	panelDescriptor.Actions = []lazycontrolplane.PanelActionInfo{testAction("refresh", "/addons/status/refresh")}
	requestedEndpoint := false
	appControl := panelTestServer(t, []lazycontrolplane.PanelInfo{panelDescriptor}, func(w http.ResponseWriter, _ *http.Request) {
		requestedEndpoint = true
		w.Header().Set("Content-Type", "text/plain")
	})
	defer appControl.Close()
	controller := testAddonsController(appControl)

	response := httptest.NewRecorder()
	if err := controller.Action(response, validActionRequest("status", "delete-everything")); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if requestedEndpoint {
		t.Fatal("unknown action ID invoked an endpoint")
	}
}

func TestDiscoverRejectsUnsafeActionDescriptors(t *testing.T) {
	tests := []struct {
		name   string
		change func(*lazycontrolplane.PanelActionInfo)
	}{
		{name: "method", change: func(action *lazycontrolplane.PanelActionInfo) {
			action.Method = http.MethodGet
			action.Pattern = "GET " + action.Path
		}},
		{name: "absolute URL", change: func(action *lazycontrolplane.PanelActionInfo) {
			action.Path = "https://attacker.example/action"
			action.Pattern = "POST " + action.Path
		}},
		{name: "pattern mismatch", change: func(action *lazycontrolplane.PanelActionInfo) { action.Pattern = "POST /different" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			panelDescriptor := testPanel("status", "/addons/status")
			action := testAction("refresh", "/addons/status/refresh")
			test.change(&action)
			panelDescriptor.Actions = []lazycontrolplane.PanelActionInfo{action}
			requestedEndpoint := false
			appControl := panelTestServer(t, []lazycontrolplane.PanelInfo{panelDescriptor}, func(http.ResponseWriter, *http.Request) {
				requestedEndpoint = true
			})
			defer appControl.Close()
			controller := testAddonsController(appControl)
			response := httptest.NewRecorder()
			if err := controller.Action(response, validActionRequest("status", "refresh")); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
			}
			if requestedEndpoint {
				t.Fatal("unsafe descriptor invoked an endpoint")
			}
		})
	}
}

func TestDiscoverSortsDeterministically(t *testing.T) {
	panels := []lazycontrolplane.PanelInfo{
		panelWithOrder(testPanel("zeta", "/addons/zeta"), 20, "Zeta"),
		panelWithOrder(testPanel("alpha", "/addons/alpha"), 10, "Alpha"),
	}
	appControl := panelTestServer(t, panels, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
	})
	defer appControl.Close()
	controller := testAddonsController(appControl)

	discovered, err := controller.discover(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(discovered) != 2 || discovered[0].ID != "alpha" || discovered[1].ID != "zeta" {
		t.Fatalf("panels = %#v, want deterministic order", discovered)
	}
}

func TestDiscoverRejectsUnsupportedSchemaAndNonLoopbackAddress(t *testing.T) {
	t.Run("schema", func(t *testing.T) {
		appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"schema":2,"panels":[]}`)
		}))
		defer appControl.Close()
		controller := testAddonsController(appControl)
		if _, err := controller.discover(t.Context()); err == nil || !strings.Contains(err.Error(), "schema 2") {
			t.Fatalf("discover error = %v, want unsupported schema", err)
		}
	})

	t.Run("non loopback", func(t *testing.T) {
		store := buildservice.NewStore(10)
		store.Update(buildservice.Snapshot{ControlPlaneAddr: "example.com:80"})
		controller := &AddonsController{Base: panel.Base{Store: store}}
		if _, err := controller.discover(t.Context()); err == nil || !strings.Contains(err.Error(), "not loopback") {
			t.Fatalf("discover error = %v, want loopback rejection", err)
		}
	})
}

func panelTestServer(t *testing.T, panels []lazycontrolplane.PanelInfo, endpoint http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == lazycontrolplane.PanelsPath {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(lazycontrolplane.PanelsResponse{Schema: 1, Panels: panels}); err != nil {
				t.Errorf("encode discovery: %v", err)
			}
			return
		}
		endpoint(w, r)
	}))
}

func testAddonsController(server *httptest.Server) *AddonsController {
	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(server.URL, "http://"),
	})
	return &AddonsController{Base: panel.Base{Store: store}}
}

func testPanel(id string, endpointPath string) lazycontrolplane.PanelInfo {
	return lazycontrolplane.PanelInfo{
		ID:          id,
		Owner:       "example.test/addon",
		Title:       "Panel " + id,
		Description: "Panel description",
		EndpointID:  id + ".endpoint",
		Pattern:     http.MethodGet + " " + endpointPath,
		Method:      http.MethodGet,
		Path:        endpointPath,
	}
}

func testAction(id string, endpointPath string) lazycontrolplane.PanelActionInfo {
	return lazycontrolplane.PanelActionInfo{
		ID:          id,
		Title:       "Action " + id,
		Description: "Action description",
		EndpointID:  id + ".endpoint",
		Pattern:     http.MethodPost + " " + endpointPath,
		Method:      http.MethodPost,
		Path:        endpointPath,
	}
}

func validActionRequest(panelID, actionID string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, panelActionPath, nil)
	replaceActionForm(request, url.Values{
		"panel":      {panelID},
		"action":     {actionID},
		"csrf_token": {actionCSRFToken},
	})
	return request
}

func replaceActionForm(request *http.Request, values url.Values) {
	encoded := values.Encode()
	request.Body = http.NoBody
	if encoded != "" {
		request.Body = io.NopCloser(strings.NewReader(encoded))
	}
	request.ContentLength = int64(len(encoded))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
}

func panelWithOrder(panel lazycontrolplane.PanelInfo, order int, title string) lazycontrolplane.PanelInfo {
	panel.Order = order
	panel.Title = title
	return panel
}
