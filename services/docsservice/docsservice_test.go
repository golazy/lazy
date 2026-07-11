package docsservice

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectLoadsLocalPackages(t *testing.T) {
	root := testModule(t)
	manager := NewManager()

	index, err := manager.Project(root)
	if err != nil {
		t.Fatal(err)
	}
	version, ok := index.Latest()
	if !ok {
		t.Fatal("latest version is missing")
	}
	if version.Module != "example.com/app" {
		t.Fatalf("module = %q, want example.com/app", version.Module)
	}
	if _, ok := version.Package("example.com/app/posts"); !ok {
		t.Fatalf("posts package missing from %#v", version.Packages)
	}
}

func TestModulesListsMainModule(t *testing.T) {
	root := testModule(t)
	manager := NewManager()

	modules, err := manager.Modules(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) == 0 {
		t.Fatal("Modules returned no modules")
	}
	if modules[0].Path != "example.com/app" || !modules[0].Main {
		t.Fatalf("first module = %#v, want main example.com/app", modules[0])
	}
}

func TestModuleProjectLoadsReplacementDependency(t *testing.T) {
	root := testModuleWithDependency(t)
	manager := NewManager()

	index, module, err := manager.ModuleProject(context.Background(), root, "example.com/lib")
	if err != nil {
		t.Fatal(err)
	}
	if module.Path != "example.com/lib" || module.Dir == "" {
		t.Fatalf("module = %#v, want replacement example.com/lib with source dir", module)
	}
	version, ok := index.Latest()
	if !ok {
		t.Fatal("latest version is missing")
	}
	if version.Module != "example.com/lib" {
		t.Fatalf("module = %q, want example.com/lib", version.Module)
	}
	if _, ok := version.Package("example.com/lib"); !ok {
		t.Fatalf("dependency package missing from %#v", version.Packages)
	}
}

func TestSourceReadsReplacementDependency(t *testing.T) {
	root := testModuleWithDependency(t)
	manager := NewManager()

	source, err := manager.Source(context.Background(), root, "example.com/lib", "lib.go", 4)
	if err != nil {
		t.Fatal(err)
	}
	if source.Path != "lib.go" {
		t.Fatalf("Path = %q, want lib.go", source.Path)
	}
	if len(source.Lines) < 4 || !source.Lines[3].Highlighted {
		t.Fatalf("line 4 is not highlighted: %#v", source.Lines)
	}
}

func TestSourceRejectsTraversal(t *testing.T) {
	root := testModule(t)
	manager := NewManager()

	if _, err := manager.Source(context.Background(), root, "", "../go.mod", 1); err == nil {
		t.Fatal("Source accepted traversal path")
	}
}

func TestSourceReadsHighlightedLines(t *testing.T) {
	root := testModule(t)
	manager := NewManager()

	source, err := manager.Source(context.Background(), root, "", "posts/doc.go", 4)
	if err != nil {
		t.Fatal(err)
	}
	if source.Path != "posts/doc.go" {
		t.Fatalf("Path = %q, want posts/doc.go", source.Path)
	}
	if len(source.Lines) < 4 || !source.Lines[3].Highlighted {
		t.Fatalf("line 4 is not highlighted: %#v", source.Lines)
	}
}

func testModule(t *testing.T) string {
	t.Helper()
	t.Setenv("GOWORK", "off")
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.24\n")
	writeFile(t, filepath.Join(root, "posts", "doc.go"), `// Package posts documents posts.
package posts

// Post is a documented type.
type Post struct{}
`)
	return root
}

func testModuleWithDependency(t *testing.T) string {
	t.Helper()
	root := testModule(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.24\n\nrequire example.com/lib v0.0.0\n\nreplace example.com/lib => ./lib\n")
	writeFile(t, filepath.Join(root, "lib", "go.mod"), "module example.com/lib\n\ngo 1.24\n")
	writeFile(t, filepath.Join(root, "lib", "lib.go"), `// Package lib documents a local replacement dependency.
package lib

// DependencyThing is documented in a dependency module.
type DependencyThing struct{}
`)
	return root
}

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
