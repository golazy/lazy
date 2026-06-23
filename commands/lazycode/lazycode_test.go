package lazycode

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteFileFormatsAndUpdatesImports(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.go")
	if err := os.WriteFile(path, []byte("package p\n\nimport \"context\"\n\nfunc F(context.Context) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := RewriteFile(path, false, func(_ *token.FileSet, file *ast.File) (bool, error) {
		changed := EnsureImport(file, "golazy.dev/lazydeps")
		if !UsesSelector(file, "context") {
			changed = RemoveImport(file, "context") || changed
		}
		return changed, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"golazy.dev/lazydeps"`) {
		t.Fatalf("file = %s", data)
	}
	if !strings.Contains(string(data), `"context"`) {
		t.Fatalf("context import removed unexpectedly: %s", data)
	}
}
