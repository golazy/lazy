package addonservice

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestInstalledAddonsFieldInConfigFunction(t *testing.T) {
	source := `package appinit
import "golazy.dev/lazyapp"
func App() *lazyapp.App { return lazyapp.New(Config()) }
func Config() lazyapp.Config { return lazyapp.Config{Name: "example"} }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "init/app.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := ensureInstalledAddonsField(fset, file)
	if err != nil || !changed {
		t.Fatalf("add field: changed=%t err=%v", changed, err)
	}
	literal, err := lazyappConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(literal.Elts) != 2 {
		t.Fatalf("config fields = %d, want 2", len(literal.Elts))
	}
	changed, err = removeInstalledAddonsField(fset, file)
	if err != nil || !changed {
		t.Fatalf("remove field: changed=%t err=%v", changed, err)
	}
	if len(literal.Elts) != 1 {
		t.Fatalf("config fields after removal = %d, want 1", len(literal.Elts))
	}
}

func TestConfigFunctionRequiresUnambiguousLiteral(t *testing.T) {
	source := `package appinit
import "golazy.dev/lazyapp"
func App() *lazyapp.App { return lazyapp.New(Config()) }
func Config() lazyapp.Config { if true { return lazyapp.Config{} }; return lazyapp.Config{} }
`
	file, err := parser.ParseFile(token.NewFileSet(), "init/app.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lazyappConfig(file); err == nil || !strings.Contains(err.Error(), "unambiguous") {
		t.Fatalf("got %v, want ambiguity error", err)
	}
}
