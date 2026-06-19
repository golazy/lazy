package reloadproxy

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestInjectScriptAddsReloadClientBeforeBodyClose(t *testing.T) {
	body := []byte("<html><body><h1>Hello</h1></body></html>")
	got := injectScript(body)
	if !bytes.Contains(got, reloadScript) {
		t.Fatalf("injectScript() did not include reload script: %s", got)
	}
	if !strings.Contains(string(got), "</script></body>") {
		t.Fatalf("injectScript() did not insert before body close: %s", got)
	}

	again := injectScript(got)
	if bytes.Count(again, []byte("<script>")) != 1 {
		t.Fatalf("injectScript() duplicated reload client: %s", again)
	}
}

func TestInjectScriptAppendsWhenBodyCloseIsMissing(t *testing.T) {
	got := injectScript([]byte("<main>Hello</main>"))
	if !bytes.HasSuffix(got, reloadScript) {
		t.Fatalf("injectScript() = %s, want reload script appended", got)
	}
}

func TestIsHTMLResponse(t *testing.T) {
	tests := map[string]bool{
		"text/html":                  true,
		"text/html; charset=utf-8":   true,
		"application/json":           false,
		"text/vnd.turbo-stream.html": false,
		"application/xhtml+xml":      false,
		"application/octet-stream":   false,
	}
	for contentType, want := range tests {
		response := &http.Response{Header: http.Header{"Content-Type": []string{contentType}}}
		if got := isHTMLResponse(response); got != want {
			t.Fatalf("isHTMLResponse(%q) = %v, want %v", contentType, got, want)
		}
	}
}
