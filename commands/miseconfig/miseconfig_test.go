package miseconfig

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveGoToolRemovesOnlyToolsGo(t *testing.T) {
	input := []byte(strings.Join([]string{
		"[tools]",
		`go = "1.26.0"`,
		`node = "24"`,
		`"aqua:getsops/sops" = "latest"`,
		"",
		"[env]",
		`go = "kept"`,
		`_.file = ".secrets/development.env"`,
		"",
	}, "\n"))

	got, ok := RemoveGoTool(input)
	if !ok {
		t.Fatal("RemoveGoTool did not report a removal")
	}
	if strings.Contains(string(got), `go = "1.26.0"`) {
		t.Fatalf("tools Go line was not removed: %s", got)
	}
	if !strings.Contains(string(got), `go = "kept"`) {
		t.Fatalf("env Go value should be kept: %s", got)
	}
	if !strings.Contains(string(got), `node = "24"`) {
		t.Fatalf("node tool should be kept: %s", got)
	}
}

func TestGoToolCheckPromptsAndRemovesGo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mise.toml")
	if err := os.WriteFile(path, []byte("[tools]\ngo = \"1.26.0\"\nnode = \"24\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := (GoToolCheck{
		Dir:    dir,
		Stdin:  strings.NewReader("yes\n"),
		Stdout: &stdout,
		Stderr: &stderr,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "go =") {
		t.Fatalf("mise.toml = %q, still contains go tool", data)
	}
	if !strings.Contains(stderr.String(), "Go already bundles multi-version support") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "removed Go from mise.toml") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestGoToolCheckDeclineLeavesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mise.toml")
	original := "[tools]\ngo = \"1.26.0\"\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	err := (GoToolCheck{
		Dir:    dir,
		Stdin:  strings.NewReader("\n"),
		Stderr: &bytes.Buffer{},
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("mise.toml = %q, want %q", data, original)
	}
}

func TestGoToolCheckDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mise.toml")
	original := "[tools]\ngo = \"1.26.0\"\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	err := (GoToolCheck{
		Dir:    dir,
		DryRun: true,
		Stdout: &stdout,
	}).Execute()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("mise.toml = %q, want %q", data, original)
	}
	if !strings.Contains(stdout.String(), "would remove Go from mise.toml") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
