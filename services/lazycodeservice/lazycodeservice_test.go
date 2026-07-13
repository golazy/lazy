package lazycodeservice_test

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golazy.dev/lazy/services/lazycodeservice"
)

func TestRewriteFileAppliesFormattedEditAndPreservesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.go")
	if err := os.WriteFile(path, []byte("package app\n\nfunc Existing() {}\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	changed, err := lazycodeservice.RewriteFile(path, false, func(_ *token.FileSet, file *ast.File) (bool, error) {
		file.Decls = append(file.Decls, &ast.FuncDecl{
			Name: ast.NewIdent("Added"),
			Type: &ast.FuncType{Params: &ast.FieldList{}},
			Body: &ast.BlockStmt{},
		})
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("changed = false")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "func Added()") {
		t.Fatalf("rewritten file =\n%s", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("mode = %o, want 640", got)
	}
}

func TestRewriteFileDryRunDoesNotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.go")
	original := []byte("package app\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := lazycodeservice.RewriteFile(path, true, func(_ *token.FileSet, file *ast.File) (bool, error) {
		return lazycodeservice.EnsureImport(file, "golazy.dev/lazydeps"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("changed = false")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("dry run wrote file:\n%s", data)
	}
}

func TestRewriteFileDetectsBaselineDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.go")
	if err := os.WriteFile(path, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := lazycodeservice.RewriteFile(path, false, func(_ *token.FileSet, file *ast.File) (bool, error) {
		if err := os.WriteFile(path, []byte("package drifted\n"), 0o644); err != nil {
			return false, err
		}
		return lazycodeservice.EnsureImport(file, "golazy.dev/lazydeps"), nil
	})
	if err == nil || !strings.Contains(err.Error(), "changed after rewrite planning") {
		t.Fatalf("RewriteFile error = %v", err)
	}
	if changed {
		t.Fatal("changed = true after baseline drift")
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "package drifted\n" {
		t.Fatalf("drifted file overwritten:\n%s", data)
	}
}

func TestRewriteFileNoSemanticChangeReturnsFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.go")
	if err := os.WriteFile(path, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := lazycodeservice.RewriteFile(path, false, func(*token.FileSet, *ast.File) (bool, error) {
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("changed = true")
	}
}
