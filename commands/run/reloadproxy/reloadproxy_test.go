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

func TestInjectScriptLeavesFragmentsAlone(t *testing.T) {
	body := []byte("<main>Hello</main>")
	got := injectScript(body)
	if string(got) != string(body) {
		t.Fatalf("injectScript() = %s, want original fragment", got)
	}
}

func TestShouldInjectReloadClientSkipsTurboFrameRequests(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "/polls/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Turbo-Frame", "poll_vote")
	response := &http.Response{
		Request: request,
		Header:  http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
	}
	if shouldInjectReloadClient(response) {
		t.Fatal("shouldInjectReloadClient() = true, want false for Turbo frame request")
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

func TestStatusPageLabelsRunFailureOutput(t *testing.T) {
	page := string(statusPage(Status{
		State:  StateRunFailed,
		Output: "panic: broken",
	}))
	if !strings.Contains(page, "<h2>Run output</h2>") {
		t.Fatalf("status page does not label run output:\n%s", page)
	}
	if strings.Contains(page, "<h2>Build output</h2>") {
		t.Fatalf("status page used build output label for run failure:\n%s", page)
	}
	if !strings.Contains(page, "panic: broken") {
		t.Fatalf("status page does not include run output:\n%s", page)
	}
}

func TestStatusPageLabelsReloadFailureOutput(t *testing.T) {
	page := string(statusPage(Status{
		State:  StateReloadFailed,
		Output: "reload views: parse failed",
	}))
	if !strings.Contains(page, "<h2>Reload output</h2>") {
		t.Fatalf("status page does not label reload output:\n%s", page)
	}
	if strings.Contains(page, "<h2>Build output</h2>") {
		t.Fatalf("status page used build output label for reload failure:\n%s", page)
	}
	if !strings.Contains(page, "reload views: parse failed") {
		t.Fatalf("status page does not include reload output:\n%s", page)
	}
}
