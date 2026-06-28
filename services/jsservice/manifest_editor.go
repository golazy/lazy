package jsservice

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golazy.dev/lazy/services/execservice"
)

type ManifestEditor struct {
	root             string
	path             string
	original         []byte
	originalManifest Manifest
	manifest         Manifest
	dirty            bool
	closed           bool

	Runner  execservice.Runner
	Mise    execservice.OutputRunner
	Bundler Bundler
}

type ManifestCloseError struct {
	Err         error
	Diff        string
	Output      string
	RollbackErr error
}

func (e *ManifestCloseError) Error() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "close lazyshaft manifest: %v", e.Err)
	if e.RollbackErr != nil {
		fmt.Fprintf(&builder, "\nrollback failed: %v", e.RollbackErr)
	}
	if strings.TrimSpace(e.Diff) != "" {
		builder.WriteString("\n\nManifest diff:\n")
		builder.WriteString(e.Diff)
	}
	if e.Output != "" {
		builder.WriteString("\n\nCommand output:\n")
		builder.WriteString(e.Output)
	}
	return builder.String()
}

func (e *ManifestCloseError) Unwrap() error {
	return e.Err
}

func OpenManifest(dir string) (*ManifestEditor, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	root, err := findAppRoot(dir)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "js.toml")
	original, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read js.toml: %w", err)
	}
	manifest, err := ParseManifest(original)
	if err != nil {
		return nil, fmt.Errorf("parse js.toml: %w", err)
	}

	return &ManifestEditor{
		root:             root,
		path:             path,
		original:         append([]byte(nil), original...),
		originalManifest: cloneManifest(manifest),
		manifest:         cloneManifest(manifest),
	}, nil
}

func (e *ManifestEditor) Manifest() Manifest {
	return cloneManifest(e.manifest)
}

func (e *ManifestEditor) Update(update func(*Manifest) error) error {
	if err := e.ensureOpen(); err != nil {
		return err
	}
	if update == nil {
		return fmt.Errorf("manifest update function is required")
	}

	next := cloneManifest(e.manifest)
	if err := update(&next); err != nil {
		return err
	}
	next = normalizeManifest(next)
	if err := ValidateManifest(next); err != nil {
		return err
	}

	e.manifest = next
	e.dirty = true
	return nil
}

func (e *ManifestEditor) AddEntrypoint(entrypoint Entrypoint) error {
	return e.Update(func(manifest *Manifest) error {
		manifest.Entrypoints = append(manifest.Entrypoints, entrypoint)
		return nil
	})
}

func (e *ManifestEditor) UpdateEntrypoint(name string, update func(*Entrypoint) error) error {
	if update == nil {
		return fmt.Errorf("entrypoint update function is required")
	}
	return e.Update(func(manifest *Manifest) error {
		index := entrypointIndex(manifest.Entrypoints, name)
		if index == -1 {
			return fmt.Errorf("entrypoint %q not found", name)
		}
		return update(&manifest.Entrypoints[index])
	})
}

func (e *ManifestEditor) RemoveEntrypoint(name string) error {
	return e.Update(func(manifest *Manifest) error {
		index := entrypointIndex(manifest.Entrypoints, name)
		if index == -1 {
			return fmt.Errorf("entrypoint %q not found", name)
		}
		manifest.Entrypoints = append(manifest.Entrypoints[:index], manifest.Entrypoints[index+1:]...)
		return nil
	})
}

func (e *ManifestEditor) Close() error {
	if err := e.ensureOpen(); err != nil {
		return err
	}

	next := e.original
	if e.dirty {
		data, err := FormatManifest(e.manifest)
		if err != nil {
			return err
		}
		next = data
	}

	snapshots, err := snapshotPaths(managedManifestPaths(e.root, e.path, e.originalManifest, e.manifest))
	if err != nil {
		return err
	}
	diff := manifestDiff(e.original, next)

	if e.dirty {
		if err := os.WriteFile(e.path, next, 0o644); err != nil {
			rollbackErr := restoreSnapshots(snapshots)
			return &ManifestCloseError{
				Err:         fmt.Errorf("write js.toml: %w", err),
				Diff:        diff,
				RollbackErr: rollbackErr,
			}
		}
	}

	var output bytes.Buffer
	command := Command{
		Dir:     e.root,
		Stdout:  &output,
		Stderr:  &output,
		Runner:  e.Runner,
		Mise:    e.Mise,
		Bundler: e.Bundler,
	}
	code, err := command.Execute()
	if err == nil && code != 0 {
		err = fmt.Errorf("lazy js failed with exit code %d", code)
	}
	if err != nil {
		rollbackErr := restoreSnapshots(snapshots)
		return &ManifestCloseError{
			Err:         err,
			Diff:        diff,
			Output:      output.String(),
			RollbackErr: rollbackErr,
		}
	}

	e.closed = true
	e.original = append(e.original[:0], next...)
	e.originalManifest = cloneManifest(e.manifest)
	e.dirty = false
	return nil
}

func (e *ManifestEditor) ensureOpen() error {
	if e == nil {
		return fmt.Errorf("manifest editor is nil")
	}
	if e.closed {
		return fmt.Errorf("manifest editor is already closed")
	}
	return nil
}

func cloneManifest(manifest Manifest) Manifest {
	clone := manifest
	clone.Entrypoints = make([]Entrypoint, len(manifest.Entrypoints))
	for index, entrypoint := range manifest.Entrypoints {
		clone.Entrypoints[index] = cloneEntrypoint(entrypoint)
	}
	return clone
}

func CloneManifest(manifest Manifest) Manifest {
	return cloneManifest(manifest)
}

func cloneEntrypoint(entrypoint Entrypoint) Entrypoint {
	clone := entrypoint
	clone.Imports = append([]string(nil), entrypoint.Imports...)
	clone.ExtraFiles = append([]string(nil), entrypoint.ExtraFiles...)
	clone.Assets = append([]string(nil), entrypoint.Assets...)
	return clone
}

func entrypointIndex(entrypoints []Entrypoint, name string) int {
	for index, entrypoint := range entrypoints {
		if entrypoint.Name == name {
			return index
		}
	}
	return -1
}

func managedManifestPaths(root string, manifestPath string, manifests ...Manifest) []string {
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		if path == "" {
			return
		}
		path = filepath.Clean(path)
		if seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}

	add(manifestPath)
	for _, manifest := range manifests {
		manifest = normalizeManifest(manifest)
		packagePath := resolvePath(root, manifest.Package)
		packageDir := filepath.Dir(packagePath)
		add(packagePath)
		for _, name := range []string{
			"package-lock.json",
			"npm-shrinkwrap.json",
			"pnpm-lock.yaml",
			"yarn.lock",
			"bun.lock",
			"bun.lockb",
		} {
			add(filepath.Join(packageDir, name))
		}
		add(resolvePath(root, manifest.Output.Importmap))
		add(resolvePath(root, manifest.Output.Dir))
	}
	sort.Strings(paths)
	return paths
}

type pathSnapshot struct {
	Path    string
	Exists  bool
	IsDir   bool
	Mode    os.FileMode
	Data    []byte
	Entries []snapshotEntry
}

type snapshotEntry struct {
	Relative string
	IsDir    bool
	Mode     os.FileMode
	Data     []byte
}

func snapshotPaths(paths []string) ([]pathSnapshot, error) {
	snapshots := make([]pathSnapshot, 0, len(paths))
	for _, path := range paths {
		snapshot, err := snapshotPath(path)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func snapshotPath(path string) (pathSnapshot, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return pathSnapshot{Path: path}, nil
	}
	if err != nil {
		return pathSnapshot{}, fmt.Errorf("snapshot %s: %w", path, err)
	}

	snapshot := pathSnapshot{
		Path:   path,
		Exists: true,
		IsDir:  info.IsDir(),
		Mode:   info.Mode().Perm(),
	}
	if !info.IsDir() {
		data, err := os.ReadFile(path)
		if err != nil {
			return pathSnapshot{}, fmt.Errorf("snapshot %s: %w", path, err)
		}
		snapshot.Data = data
		return snapshot, nil
	}

	if err := filepath.WalkDir(path, func(candidate string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if candidate == path {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(path, candidate)
		if err != nil {
			return err
		}
		item := snapshotEntry{
			Relative: relative,
			IsDir:    entry.IsDir(),
			Mode:     info.Mode().Perm(),
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(candidate)
			if err != nil {
				return err
			}
			item.Data = data
		}
		snapshot.Entries = append(snapshot.Entries, item)
		return nil
	}); err != nil {
		return pathSnapshot{}, fmt.Errorf("snapshot %s: %w", path, err)
	}
	return snapshot, nil
}

func restoreSnapshots(snapshots []pathSnapshot) error {
	for index := len(snapshots) - 1; index >= 0; index-- {
		if err := restoreSnapshot(snapshots[index]); err != nil {
			return err
		}
	}
	return nil
}

func restoreSnapshot(snapshot pathSnapshot) error {
	if err := os.RemoveAll(snapshot.Path); err != nil {
		return fmt.Errorf("remove %s before restore: %w", snapshot.Path, err)
	}
	if !snapshot.Exists {
		return nil
	}
	if !snapshot.IsDir {
		if err := os.MkdirAll(filepath.Dir(snapshot.Path), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(snapshot.Path), err)
		}
		if err := os.WriteFile(snapshot.Path, snapshot.Data, snapshot.Mode); err != nil {
			return fmt.Errorf("restore %s: %w", snapshot.Path, err)
		}
		return nil
	}

	if err := os.MkdirAll(snapshot.Path, snapshot.Mode); err != nil {
		return fmt.Errorf("restore %s: %w", snapshot.Path, err)
	}
	for _, entry := range snapshot.Entries {
		target := filepath.Join(snapshot.Path, entry.Relative)
		if entry.IsDir {
			if err := os.MkdirAll(target, entry.Mode); err != nil {
				return fmt.Errorf("restore %s: %w", target, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, entry.Data, entry.Mode); err != nil {
			return fmt.Errorf("restore %s: %w", target, err)
		}
	}
	return nil
}

func manifestDiff(before, after []byte) string {
	if bytes.Equal(before, after) {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("--- js.toml\n")
	builder.WriteString("+++ js.toml\n")
	builder.WriteString("@@\n")
	for _, line := range diffLines(before) {
		builder.WriteByte('-')
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	for _, line := range diffLines(after) {
		builder.WriteByte('+')
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func diffLines(data []byte) []string {
	text := strings.TrimRight(string(data), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
