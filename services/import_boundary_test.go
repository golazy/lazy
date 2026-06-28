package services_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestServicesDoNotImportCommands(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	servicesRoot := filepath.Dir(filename)
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(servicesRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}

		file, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if importPath == "golazy.dev/lazy/commands" || strings.HasPrefix(importPath, "golazy.dev/lazy/commands/") {
				relative, relErr := filepath.Rel(servicesRoot, path)
				if relErr != nil {
					relative = path
				}
				t.Errorf("%s imports %s", filepath.ToSlash(relative), importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
