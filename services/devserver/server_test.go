package devserver

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInjectScriptAddsExternalPanelClientBeforeBodyClose(t *testing.T) {
	body := []byte("<html><body><h1>Hello</h1></body></html>")
	got := injectScript(body)
	if !bytes.Contains(got, clientScript) {
		t.Fatalf("injectScript() did not include panel client: %s", got)
	}
	if !bytes.Contains(got, []byte(`src="/_golazy/assets/panel.js"`)) {
		t.Fatalf("injectScript() did not use external panel client: %s", got)
	}
	if !bytes.Contains(got, []byte(`type="module"`)) {
		t.Fatalf("injectScript() did not load the panel client as a module: %s", got)
	}
	if bytes.Contains(got, []byte("new EventSource")) {
		t.Fatalf("injectScript() embedded JavaScript: %s", got)
	}

	again := injectScript(got)
	if bytes.Count(again, []byte(PanelClientPath)) != 1 {
		t.Fatalf("injectScript() duplicated panel client: %s", again)
	}
}

func TestShouldInjectClientSkipsTurboFramesAndNonSuccess(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/polls/1", nil)
	request.Header.Set("Turbo-Frame", "poll_vote")
	response := &http.Response{
		StatusCode: http.StatusOK,
		Request:    request,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
	}
	if shouldInjectClient(response) {
		t.Fatal("shouldInjectClient() = true, want false for Turbo frame request")
	}

	response.Request = httptest.NewRequest(http.MethodGet, "/polls/1", nil)
	response.StatusCode = http.StatusUnauthorized
	if shouldInjectClient(response) {
		t.Fatal("shouldInjectClient() = true, want false for non-2xx response")
	}
}

func TestServerRoutesPanelPrefixBeforeProxy(t *testing.T) {
	panel := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "panel")
	})
	server, err := New("127.0.0.1:0", panel, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy/", nil))
	if got := response.Body.String(); got != "panel" {
		t.Fatalf("panel body = %q, want panel", got)
	}
}

func TestServerNormalizesPanelRootSlash(t *testing.T) {
	panel := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_golazy" {
			t.Fatalf("panel path = %q, want /_golazy", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, "panel")
	})
	server, err := New("127.0.0.1:0", panel, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_golazy/", nil))
	if got := response.Body.String(); got != "panel" {
		t.Fatalf("panel body = %q, want panel", got)
	}
}

func TestServerAddsRequestTraceHeadersToProxiedRequests(t *testing.T) {
	headers := make(chan http.Header, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Clone()
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer backend.Close()

	server, err := New("127.0.0.1:0", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())
	server.SetTarget(backend.URL)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/posts", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	got := <-headers
	if requestIDFromHeader(got.Get(requestIDHeader)) == "" {
		t.Fatalf("X-Request-ID = %q, want generated request id", got.Get(requestIDHeader))
	}
	if !validTraceparent(got.Get("traceparent")) {
		t.Fatalf("traceparent = %q, want generated traceparent", got.Get("traceparent"))
	}
}

func TestServerPreservesExistingRequestTraceHeaders(t *testing.T) {
	headers := make(chan http.Header, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Clone()
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer backend.Close()

	server, err := New("127.0.0.1:0", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())
	server.SetTarget(backend.URL)

	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	request := httptest.NewRequest(http.MethodGet, "/posts", nil)
	request.Header.Set(requestIDHeader, "browser-req-1")
	request.Header.Set("traceparent", traceparent)
	request.Header.Set("tracestate", "vendor=value")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	got := <-headers
	if got.Get(requestIDHeader) != "browser-req-1" {
		t.Fatalf("X-Request-ID = %q, want browser-req-1", got.Get(requestIDHeader))
	}
	if got.Get("traceparent") != traceparent {
		t.Fatalf("traceparent = %q, want %q", got.Get("traceparent"), traceparent)
	}
	if got.Get("tracestate") != "vendor=value" {
		t.Fatalf("tracestate = %q, want vendor=value", got.Get("tracestate"))
	}
}
