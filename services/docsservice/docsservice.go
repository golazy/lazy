package docsservice

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golazy.dev/lazydoc"
)

type contextKey struct{}

type Manager struct{}

type Module struct {
	Path    string
	Version string
	Dir     string
	Main    bool
}

type SourceFile struct {
	Path  string
	Line  int
	Lines []SourceLine
}

type SourceLine struct {
	Number      int
	Text        string
	Highlighted bool
}

func NewManager() *Manager {
	return &Manager{}
}

func WithManager(ctx context.Context, manager *Manager) context.Context {
	if manager == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, manager)
}

func ManagerFromContext(ctx context.Context) (*Manager, bool) {
	manager, ok := ctx.Value(contextKey{}).(*Manager)
	return manager, ok && manager != nil
}

func (m *Manager) Project(root string) (*lazydoc.Index, error) {
	root, err := cleanRoot(root)
	if err != nil {
		return nil, err
	}
	return lazydoc.LoadDir(root, "local")
}

func (m *Manager) ModuleProject(ctx context.Context, root string, modulePath string) (*lazydoc.Index, Module, error) {
	dir, module, err := m.moduleRoot(ctx, root, modulePath)
	if err != nil {
		return nil, Module{}, err
	}
	index, err := lazydoc.LoadDir(dir, "local")
	if err != nil {
		return nil, Module{}, err
	}
	if module.Path == "" {
		if version, ok := index.Latest(); ok {
			module.Path = version.Module
		}
		module.Dir = dir
		module.Main = true
	}
	return index, module, nil
}

func (m *Manager) Modules(ctx context.Context, root string) ([]Module, error) {
	root, err := cleanRoot(root)
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all")
	cmd.Dir = root
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return nil, fmt.Errorf("list Go modules: %w: %s", err, message)
		}
		return nil, fmt.Errorf("list Go modules: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(out))
	var modules []Module
	for {
		var raw moduleJSON
		if err := decoder.Decode(&raw); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("parse Go module list: %w", err)
		}
		modules = append(modules, raw.module())
	}
	return modules, nil
}

func (m *Manager) Source(ctx context.Context, root string, modulePath string, file string, line int) (*SourceFile, error) {
	root, _, err := m.moduleRoot(ctx, root, modulePath)
	if err != nil {
		return nil, err
	}
	file = strings.TrimSpace(file)
	if file == "" {
		return nil, fmt.Errorf("source file is not available")
	}
	clean := filepath.Clean(filepath.FromSlash(file))
	if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return nil, fmt.Errorf("source file must stay inside the selected module")
	}
	path := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return nil, fmt.Errorf("source file must stay inside the selected module")
	}
	data, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read source file: %w", err)
	}
	defer data.Close()

	if line <= 0 {
		line = 1
	}
	source := &SourceFile{Path: filepath.ToSlash(rel), Line: line}
	scanner := bufio.NewScanner(data)
	for number := 1; scanner.Scan(); number++ {
		source.Lines = append(source.Lines, SourceLine{
			Number:      number,
			Text:        scanner.Text(),
			Highlighted: number == line,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read source file: %w", err)
	}
	return source, nil
}

func (m *Manager) moduleRoot(ctx context.Context, root string, modulePath string) (string, Module, error) {
	root, err := cleanRoot(root)
	if err != nil {
		return "", Module{}, err
	}
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return root, Module{Dir: root, Main: true}, nil
	}
	modules, err := m.Modules(ctx, root)
	if err != nil {
		return "", Module{}, err
	}
	for _, module := range modules {
		if module.Path != modulePath {
			continue
		}
		if module.Dir == "" {
			return "", Module{}, fmt.Errorf("module source is not available for %s", module.Path)
		}
		dir, err := cleanRoot(module.Dir)
		if err != nil {
			return "", Module{}, err
		}
		module.Dir = dir
		return dir, module, nil
	}
	return "", Module{}, fmt.Errorf("module %q is not in the project dependency list", modulePath)
}

func cleanRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("project root is not available")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("inspect project root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project root is not a directory")
	}
	return abs, nil
}

type moduleJSON struct {
	Path    string      `json:"Path"`
	Version string      `json:"Version"`
	Dir     string      `json:"Dir"`
	Main    bool        `json:"Main"`
	Replace *moduleJSON `json:"Replace"`
}

func (m moduleJSON) module() Module {
	module := Module{
		Path:    m.Path,
		Version: m.Version,
		Dir:     m.Dir,
		Main:    m.Main,
	}
	if module.Dir == "" && m.Replace != nil {
		module.Dir = m.Replace.Dir
	}
	return module
}
