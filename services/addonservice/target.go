package addonservice

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// validateAddonTarget ensures a planned application-relative file cannot
// escape through a symlinked parent directory. The check is repeated while a
// plan is applied because the filesystem may have changed since planning.
func validateAddonTarget(root, name string) error {
	if !validFilePath(name) {
		return fmt.Errorf("add-on target %q is invalid", name)
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve add-on root: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return fmt.Errorf("resolve add-on root: %w", err)
	}
	rootInfo, err := os.Stat(realRoot)
	if err != nil {
		return fmt.Errorf("inspect add-on root: %w", err)
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("add-on root is not a directory")
	}

	parent := filepath.Dir(filepath.Join(rootPath, filepath.FromSlash(name)))
	for {
		info, statErr := os.Lstat(parent)
		if statErr == nil {
			if !info.IsDir() && info.Mode()&fs.ModeSymlink == 0 {
				return fmt.Errorf("add-on target %q has non-directory parent %s", name, parent)
			}
			realParent, evalErr := filepath.EvalSymlinks(parent)
			if evalErr != nil {
				return fmt.Errorf("resolve parent for add-on target %q: %w", name, evalErr)
			}
			if !pathWithin(realRoot, realParent) {
				return fmt.Errorf("add-on target %q resolves outside the application root", name)
			}
			return validateAddonLeaf(rootPath, name)
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("inspect parent for add-on target %q: %w", name, statErr)
		}
		next := filepath.Dir(parent)
		if next == parent {
			return fmt.Errorf("find existing parent for add-on target %q", name)
		}
		parent = next
	}
}

func validateAddonLeaf(root, name string) error {
	filename := filepath.Join(root, filepath.FromSlash(name))
	info, err := os.Lstat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect add-on target %q: %w", name, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("add-on target %q is a symbolic link", name)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("add-on target %q is not a regular file", name)
	}
	return nil
}

// readAddonTarget opens and verifies a regular target before reading it. The
// opened file and the path's lstat identity must match, so a leaf symlink or a
// concurrent path replacement is rejected without following that path for a
// later write.
func readAddonTarget(root, name string) (data []byte, mode os.FileMode, exists bool, err error) {
	if err := validateAddonTarget(root, name); err != nil {
		return nil, 0, false, err
	}
	filename := filepath.Join(root, filepath.FromSlash(name))
	file, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, fmt.Errorf("open add-on target %q: %w", name, err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, 0, false, fmt.Errorf("inspect opened add-on target %q: %w", name, err)
	}
	leaf, err := os.Lstat(filename)
	if err != nil {
		return nil, 0, false, fmt.Errorf("reinspect add-on target %q: %w", name, err)
	}
	if leaf.Mode()&fs.ModeSymlink != 0 || !leaf.Mode().IsRegular() || !opened.Mode().IsRegular() || !os.SameFile(opened, leaf) {
		return nil, 0, false, fmt.Errorf("add-on target %q changed or is not a regular file", name)
	}
	data, err = io.ReadAll(file)
	if err != nil {
		return nil, 0, false, fmt.Errorf("read add-on target %q: %w", name, err)
	}
	return data, opened.Mode().Perm(), true, nil
}
