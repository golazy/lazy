package run

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
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
	t.Setenv("ADDR", "127.0.0.1:4000")
	t.Setenv("PORT", "5000")
	if got, want := publicListenAddr(), "127.0.0.1:4000"; got != want {
		t.Fatalf("publicListenAddr() = %q, want %q", got, want)
	}
}

func TestPublicListenAddrUsesPORT(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("PORT", "5000")
	if got, want := publicListenAddr(), ":5000"; got != want {
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
