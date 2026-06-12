package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := execute([]string{"--version"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if got, want := stdout.String(), "lazy "+currentVersion()+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestNewRequiresModuleName(t *testing.T) {
	var stderr bytes.Buffer

	if code := execute([]string{"new"}, nil, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lazy new <module>") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
