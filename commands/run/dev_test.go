package run

import (
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
