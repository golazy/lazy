package devserver

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"golazy.dev/lazy/services/customcertservice"
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
	for _, want := range []string{
		`id="golazy-dev-panel-root"`,
		`id="golazy-dev-panel-padding"`,
		`id="golazy-dev-panel"`,
		`id="golazy-dev-panel-launcher"`,
		`iframe src="/_golazy/"`,
		`img src="/_golazy/assets/logo-square.svg"`,
		`hidden`,
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Fatalf("injectScript() missing %q: %s", want, got)
		}
	}

	again := injectScript(got)
	if bytes.Count(again, []byte(PanelClientPath)) != 1 {
		t.Fatalf("injectScript() duplicated panel client: %s", again)
	}
	if bytes.Count(again, []byte(`id="`+PanelClientRootID+`"`)) != 1 {
		t.Fatalf("injectScript() duplicated panel root: %s", again)
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
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://lazy.test/_golazy/", nil))
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
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://lazy.test/_golazy/", nil))
	if got := response.Body.String(); got != "panel" {
		t.Fatalf("panel body = %q, want panel", got)
	}
}

func TestServerServesExtensionHandshakeBeforePanel(t *testing.T) {
	panel := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("panel handled extension handshake path %q", r.URL.Path)
	})
	server, err := New("127.0.0.1:0", panel, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())

	request := httptest.NewRequest(http.MethodGet, "https://lazy.test"+ExtensionHandshakePath, nil)
	request.Header.Set("Origin", "chrome-extension://golazy")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if got := response.Body.String(); got != ExtensionHandshakeBody {
		t.Fatalf("body = %q, want %q", got, ExtensionHandshakeBody)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestServerServesDevToolsWorkspaceBeforeProxy(t *testing.T) {
	var panelPath string
	panel := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panelPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{"workspace":{"root":"/app/app/js","uuid":"1f95f222-3d25-5c10-a871-82fb1634e0e1"}}`)
	})
	backendHit := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendHit = true
		_, _ = fmt.Fprint(w, "backend")
	}))
	defer backend.Close()
	server, err := New("127.0.0.1:0", panel, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())
	server.SetTarget(backend.URL)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://lazy.test"+DevToolsWorkspacePath, nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if panelPath != DevToolsWorkspacePath {
		t.Fatalf("panel path = %q, want %q", panelPath, DevToolsWorkspacePath)
	}
	if backendHit {
		t.Fatal("well-known DevTools workspace request was proxied to the app")
	}
	if !strings.Contains(response.Body.String(), `"workspace"`) {
		t.Fatalf("body = %q, want workspace JSON", response.Body.String())
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
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://lazy.test/posts", nil))

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

	request.URL.Scheme = "https"
	request.TLS = &tls.ConnectionState{}
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

func TestServerServesCertificateWelcomeOnHTTP(t *testing.T) {
	paths := testCertificatePaths(t)
	server, err := New("127.0.0.1:0", nil, nil, WithCertificatePaths(paths))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())

	request := httptest.NewRequest(http.MethodGet, "http://dev.local:3000/posts?filter=all", nil)
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0)")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	body := response.Body.String()
	for _, want := range []string{
		"Welcome to GoLazy local HTTPS",
		"Install on macOS",
		"Download certificate authority",
		"dev.local",
		"Do not share these files",
		paths.Certificate,
		paths.PrivateKey,
		"https://dev.local:3000/posts?filter=all",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("welcome page missing %q:\n%s", want, body)
		}
	}
}

func TestServerServesCertificateWelcomeCommandCopyButtons(t *testing.T) {
	paths := testCertificatePaths(t)
	server, err := New("127.0.0.1:0", nil, nil, WithCertificatePaths(paths))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())

	tests := []struct {
		name    string
		target  string
		command string
	}{
		{
			name:    "windows",
			target:  "http://lazy.test/?os=windows",
			command: "certmgr.msc",
		},
		{
			name:    "linux",
			target:  "http://lazy.test/?os=linux",
			command: "certutil -d sql:$HOME/.pki/nssdb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tt.target, nil))

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			body := response.Body.String()
			for _, want := range []string{
				`class="copy-button"`,
				`data-copy="` + tt.command,
				tt.command,
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("welcome page missing %q:\n%s", want, body)
				}
			}
		})
	}
}

func TestServerServesCertificateDownloadOnHTTP(t *testing.T) {
	paths := testCertificatePaths(t)
	server, err := New("127.0.0.1:0", nil, nil, WithCertificatePaths(paths))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://lazy.test"+CertificateDownloadPath, nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "application/x-x509-ca-cert" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(response.Body.String(), "BEGIN CERTIFICATE") {
		t.Fatalf("certificate download did not contain PEM: %q", response.Body.String())
	}
	if _, err := customcertservice.LoadOrCreate(paths); err != nil {
		t.Fatalf("download did not create reusable authority: %v", err)
	}
}

func TestServerServesHTTPAndHTTPSOnSamePort(t *testing.T) {
	paths := testCertificatePaths(t)
	panel := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "panel")
	})
	server, err := New("127.0.0.1:0", panel, nil, WithCertificatePaths(paths))
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())

	httpResponse, err := http.Get("http://" + server.Addr() + "/")
	if err != nil {
		t.Fatal(err)
	}
	httpBody, err := io.ReadAll(httpResponse.Body)
	_ = httpResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(httpBody, []byte("Welcome to GoLazy local HTTPS")) {
		t.Fatalf("HTTP body = %q, want certificate welcome page", string(httpBody))
	}

	authority, err := customcertservice.LoadOrCreate(paths)
	if err != nil {
		t.Fatal(err)
	}
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(authority.CertificatePEM()) {
		t.Fatal("failed to append generated CA")
	}
	client := &http.Client{Transport: &http.Transport{
		ForceAttemptHTTP2: true,
		TLSClientConfig:   &tls.Config{RootCAs: rootCAs},
	}}
	httpsResponse, err := client.Get("https://" + server.Addr() + HTTPSProbePath)
	if err != nil {
		t.Fatal(err)
	}
	_ = httpsResponse.Body.Close()
	if httpsResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("HTTPS probe status = %d, want 204", httpsResponse.StatusCode)
	}
	if httpsResponse.ProtoMajor != 2 {
		t.Fatalf("HTTPS protocol = %s, want HTTP/2", httpsResponse.Proto)
	}
}

func testCertificatePaths(t *testing.T) customcertservice.Paths {
	t.Helper()
	dir := t.TempDir()
	return customcertservice.Paths{
		Dir:         dir,
		Certificate: filepath.Join(dir, customcertservice.CertificateFileName),
		PrivateKey:  filepath.Join(dir, customcertservice.PrivateKeyFileName),
	}
}
