package lazycode

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strconv"
)

type RewriteFunc func(*token.FileSet, *ast.File) (bool, error)

func RewriteFile(path string, dryRun bool, rewrite RewriteFunc) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, data, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	changed, err := rewrite(fileSet, file)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}

	var formatted bytes.Buffer
	if err := format.Node(&formatted, fileSet, file); err != nil {
		return false, fmt.Errorf("format %s: %w", path, err)
	}
	if bytes.Equal(data, formatted.Bytes()) {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, formatted.Bytes(), info.Mode().Perm()); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

func EnsureImport(file *ast.File, importPath string) bool {
	if importPath == "" || HasImport(file, importPath) {
		return false
	}
	spec := &ast.ImportSpec{
		Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(importPath)},
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		gen.Specs = append(gen.Specs, spec)
		return true
	}
	file.Decls = append([]ast.Decl{
		&ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{spec}},
	}, file.Decls...)
	return true
}

func RemoveImport(file *ast.File, importPath string) bool {
	changed := false
	var decls []ast.Decl
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			decls = append(decls, decl)
			continue
		}
		var specs []ast.Spec
		for _, spec := range gen.Specs {
			imported, ok := spec.(*ast.ImportSpec)
			if !ok {
				specs = append(specs, spec)
				continue
			}
			path, err := strconv.Unquote(imported.Path.Value)
			if err == nil && path == importPath {
				changed = true
				continue
			}
			specs = append(specs, spec)
		}
		if len(specs) == 0 {
			changed = true
			continue
		}
		gen.Specs = specs
		decls = append(decls, gen)
	}
	if changed {
		file.Decls = decls
	}
	return changed
}

func HasImport(file *ast.File, importPath string) bool {
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err == nil && path == importPath {
			return true
		}
	}
	return false
}

func UsesSelector(file *ast.File, packageName string) bool {
	used := false
	ast.Inspect(file, func(node ast.Node) bool {
		if used {
			return false
		}
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if ok && ident.Name == packageName {
			used = true
			return false
		}
		return true
	})
	return used
}
