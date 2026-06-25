package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeListenAddr(t *testing.T) {
	tests := map[string]string{
		"3000":           ":3000",
		":3000":          ":3000",
		"127.0.0.1:4000": "127.0.0.1:4000",
	}
	for input, want := range tests {
		if got := normalizeListenAddr(input); got != want {
			t.Fatalf("normalizeListenAddr(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPublicListenAddrPrefersADDR(t *testing.T) {
	if got, want := publicListenAddr("127.0.0.1:4000", 5000), "127.0.0.1:4000"; got != want {
		t.Fatalf("publicListenAddr() = %q, want %q", got, want)
	}
}

func TestPublicListenAddrUsesPORT(t *testing.T) {
	if got, want := publicListenAddr("", 5000), ":5000"; got != want {
		t.Fatalf("publicListenAddr() = %q, want %q", got, want)
	}
}

func TestPublicListenAddrUsesPORTOverDefaultADDR(t *testing.T) {
	if got, want := publicListenAddr(defaultListenAddr, 5000), ":5000"; got != want {
		t.Fatalf("publicListenAddr() = %q, want %q", got, want)
	}
}

func TestPublicListenAddrUsesDefaultADDRWhenPORTUnset(t *testing.T) {
	if got, want := publicListenAddr(defaultListenAddr, 0), defaultListenAddr; got != want {
		t.Fatalf("publicListenAddr() = %q, want %q", got, want)
	}
}

func TestDrainChangesDeduplicatesBufferedChanges(t *testing.T) {
	changes := make(chan []string, 2)
	changes <- []string{"app/views/index.html.tpl", "go.mod"}
	changes <- []string{"go.mod", "app/controllers/home.go"}

	got := drainChanges(changes, []string{"go.mod"})
	want := []string{"go.mod", "app/views/index.html.tpl", "app/controllers/home.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("drainChanges() = %#v, want %#v", got, want)
	}
}

func TestJavaScriptAssetGenerationMode(t *testing.T) {
	dir := t.TempDir()
	if got := javaScriptAssetGenerationMode(dir, nil); got != javaScriptAssetNone {
		t.Fatalf("mode without js.toml = %v, want none", got)
	}
	writeFile(t, filepath.Join(dir, "js.toml"), "[entrypoint.turbo]\nmodule = \"@hotwired/turbo\"\n")

	tests := []struct {
		name    string
		changed []string
		want    javaScriptAssetMode
	}{
		{name: "initial build", changed: nil, want: javaScriptAssetFull},
		{name: "manifest", changed: []string{"js.toml"}, want: javaScriptAssetFull},
		{name: "package metadata", changed: []string{"package.json"}, want: javaScriptAssetFull},
		{name: "lockfile", changed: []string{"package-lock.json"}, want: javaScriptAssetFull},
		{name: "app entry", changed: []string{"app/js/app.js"}, want: javaScriptAssetBundle},
		{name: "app controller", changed: []string{"app/js/controllers/hello_controller.js"}, want: javaScriptAssetBundle},
		{name: "app view", changed: []string{"app/views/home/index.html.tpl"}, want: javaScriptAssetNone},
		{name: "generated importmap", changed: []string{"app/public/assets/importmap.json"}, want: javaScriptAssetNone},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := javaScriptAssetGenerationMode(dir, test.changed); got != test.want {
				t.Fatalf("mode = %v, want %v", got, test.want)
			}
		})
	}
}

func TestOnlyGeneratedJavaScriptOutputs(t *testing.T) {
	if !onlyGeneratedJavaScriptOutputs([]string{
		"app/public/assets/importmap.json",
		"app/public/assets/lazyshaft/app/app-ZWUSRUPQ.js",
	}) {
		t.Fatal("generated JavaScript outputs were not ignored")
	}
	if onlyGeneratedJavaScriptOutputs([]string{
		"app/public/assets/importmap.json",
		"app/js/app.js",
	}) {
		t.Fatal("mixed generated and source changes should not be ignored")
	}
	if onlyGeneratedJavaScriptOutputs(nil) {
		t.Fatal("empty changes should not be ignored")
	}
}

func TestClassifyDevelopmentChange(t *testing.T) {
	tests := []struct {
		name       string
		viewPath   string
		publicPath string
		changed    []string
		want       devChangeAction
	}{
		{name: "default view", changed: []string{"app/views/home/index.html.tpl"}, want: devChangeReloadViews},
		{name: "default public", changed: []string{"app/public/styles.css"}, want: devChangeReloadBrowser},
		{name: "view and public", changed: []string{"app/views/home/index.html.tpl", "app/public/styles.css"}, want: devChangeReloadViews},
		{name: "custom view", viewPath: "views", changed: []string{"views/pages/index.html.tpl"}, want: devChangeReloadViews},
		{name: "custom public", publicPath: "public_files", changed: []string{"public_files/styles.css"}, want: devChangeReloadBrowser},
		{name: "go source", changed: []string{"app/controllers/home.go"}, want: devChangeRebuild},
		{name: "view and go source", changed: []string{"app/views/home/index.html.tpl", "app/controllers/home.go"}, want: devChangeRebuild},
		{name: "outside absolute view path", viewPath: "/tmp/views", changed: []string{"app/views/home/index.html.tpl"}, want: devChangeRebuild},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyDevelopmentChange(test.viewPath, test.publicPath, test.changed); got != test.want {
				t.Fatalf("classifyDevelopmentChange() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestReloadViewsPostsToLazyDevControlPlane(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/views" {
			t.Fatalf("path = %s, want reload path", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, "reload views ok\n")
	}))
	defer server.Close()

	result := reloadViews(context.Background(), strings.TrimPrefix(server.URL, "http://"))
	if result.Err != nil {
		t.Fatalf("reloadViews() error = %v", result.Err)
	}
	if got, want := result.Output, "reload views ok\n"; got != want {
		t.Fatalf("Output = %q, want %q", got, want)
	}
	if result.Duration <= 0 {
		t.Fatal("Duration was not recorded")
	}
}

func TestReloadViewsReportsResponseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "parse failed", http.StatusInternalServerError)
	}))
	defer server.Close()

	result := reloadViews(context.Background(), strings.TrimPrefix(server.URL, "http://"))
	if result.Err == nil {
		t.Fatal("reloadViews() error is nil")
	}
	if !strings.Contains(result.Output, "parse failed") {
		t.Fatalf("Output = %q, want parse failure", result.Output)
	}
}

func TestShouldExitAfterApplicationDone(t *testing.T) {
	if !shouldExitAfterApplicationDone(context.Background(), nil) {
		t.Fatal("clean application exit should stop lazy")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !shouldExitAfterApplicationDone(ctx, errors.New("application still shutting down")) {
		t.Fatal("canceled lazy context should stop lazy after application exit")
	}

	if shouldExitAfterApplicationDone(context.Background(), errors.New("application crashed")) {
		t.Fatal("unexpected application crash should leave lazy running")
	}
}

func TestStartupOutputBuffersUntilAttached(t *testing.T) {
	output := &startupOutput{}
	if _, err := output.Stderr().Write([]byte("panic: broken\n")); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "panic: broken\n" {
		t.Fatalf("captured output = %q, want panic", got)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	output.Attach(&stdout, &stderr)
	if got := stderr.String(); got != "panic: broken\n" {
		t.Fatalf("attached stderr = %q, want buffered panic", got)
	}
	if got := output.String(); got != "" {
		t.Fatalf("captured output after attach = %q, want empty", got)
	}
	if _, err := output.Stdout().Write([]byte("live\n")); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "live\n" {
		t.Fatalf("stdout after attach = %q, want live output", got)
	}
}
