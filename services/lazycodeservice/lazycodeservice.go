package lazycodeservice

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"

	"golazy.dev/lazycode"
	"golazy.dev/lazycode/gocode"
)

type RewriteFunc = gocode.RewriteFunc

func RewriteFile(path string, dryRun bool, rewrite RewriteFunc) (bool, error) {
	root := filepath.Dir(path)
	name := filepath.Base(path)
	workspace, err := lazycode.Load(root, name)
	if err != nil {
		return false, err
	}
	result, err := workspace.Plan(gocode.Rewrite(name, rewrite))
	if err != nil {
		return false, err
	}
	if !result.Changed() {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	if len(result.Files) != 1 || result.Files[0].Path != name || result.Files[0].Kind != lazycode.EditUpdate {
		return false, fmt.Errorf("lazycode: rewrite %s produced an unexpected changeset", path)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s before write: %w", path, err)
	}
	if !result.Files[0].VerifyBaseline(current, true) {
		return false, fmt.Errorf("lazycode: %s changed after rewrite planning", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, result.Files[0].After, info.Mode().Perm()); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

func EnsureImport(file *ast.File, importPath string) bool {
	return gocode.EnsureImport(file, importPath)
}

func RemoveImport(file *ast.File, importPath string) bool {
	return gocode.RemoveImport(file, importPath)
}

func HasImport(file *ast.File, importPath string) bool {
	return gocode.HasImport(file, importPath)
}

func UsesSelector(file *ast.File, packageName string) bool {
	return gocode.UsesSelector(file, packageName)
}
