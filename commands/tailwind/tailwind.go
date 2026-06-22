package tailwindcommand

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golazy.dev/lazy/commands"
)

const (
	defaultAppInput   = "app/styles/application.css"
	defaultAppOutput  = "app/public/styles.css"
	defaultRootInput  = "styles/application.css"
	defaultRootOutput = "public/styles.css"
)

var requiredPackages = []string{"@tailwindcss/cli", "tailwindcss"}

type Command struct {
	Dir    string
	Input  string
	Output string
	Watch  bool

	Stdout io.Writer
	Stderr io.Writer
	Runner commands.Runner
}

func (c Command) Execute() (int, error) {
	stdout := c.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := c.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	dir := c.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return 1, fmt.Errorf("get working directory: %w", err)
		}
	}

	root, err := findAppRoot(dir)
	if err != nil {
		return 1, err
	}

	input := c.Input
	if strings.TrimSpace(input) == "" {
		input = defaultInput(root)
	}
	output := c.Output
	if strings.TrimSpace(output) == "" {
		output = defaultOutput(root)
	}

	inputPath := resolvePath(root, input)
	outputPath := resolvePath(root, output)
	if samePath(inputPath, outputPath) {
		return 1, fmt.Errorf("Tailwind input and output must be different paths")
	}

	fmt.Fprintln(stdout, "* Preparing Tailwind stylesheet")
	if err := ensureInput(inputPath, outputPath); err != nil {
		return 1, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return 1, fmt.Errorf("create Tailwind output directory: %w", err)
	}

	packagePath := filepath.Join(root, "package.json")
	fmt.Fprintln(stdout, "* Preparing Tailwind dependencies")
	if _, err := ensurePackageDevDependencies(packagePath, requiredPackages); err != nil {
		return 1, err
	}

	runner := c.Runner
	if runner == nil {
		runner = commands.Exec
	}

	packageManager, installArgs := detectPackageManager(root)
	fmt.Fprintln(stdout, "* Installing Tailwind dependencies")
	if err := runner(packageManager, installArgs, commands.Options{
		Dir:    root,
		Stdout: stdout,
		Stderr: stderr,
	}); err != nil {
		var processExit *commands.ExitError
		if errors.As(err, &processExit) {
			return processExit.Code, nil
		}
		return 1, fmt.Errorf("%s %v: %w", packageManager, installArgs, err)
	}

	runCommand, runArgs := tailwindRunCommand(packageManager, root, inputPath, outputPath, c.Watch)
	if c.Watch {
		fmt.Fprintln(stdout, "* Watching Tailwind stylesheet")
	} else {
		fmt.Fprintln(stdout, "* Building Tailwind stylesheet")
	}
	if err := runner(runCommand, runArgs, commands.Options{
		Dir:    root,
		Stdout: stdout,
		Stderr: stderr,
	}); err != nil {
		var processExit *commands.ExitError
		if errors.As(err, &processExit) {
			return processExit.Code, nil
		}
		return 1, fmt.Errorf("%s %v: %w", runCommand, runArgs, err)
	}

	return 0, nil
}

func defaultInput(root string) string {
	if dirExists(filepath.Join(root, "app", "public")) {
		return defaultAppInput
	}
	return defaultRootInput
}

func defaultOutput(root string) string {
	if dirExists(filepath.Join(root, "app", "public")) {
		return defaultAppOutput
	}
	return defaultRootOutput
}

func ensureInput(inputPath string, outputPath string) error {
	if fileExists(inputPath) {
		return nil
	}

	var builder strings.Builder
	builder.WriteString("@import \"tailwindcss\";\n")
	if data, err := os.ReadFile(outputPath); err == nil && strings.TrimSpace(string(data)) != "" {
		builder.WriteByte('\n')
		builder.WriteString(strings.TrimRight(string(data), "\n"))
		builder.WriteByte('\n')
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read existing stylesheet: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		return fmt.Errorf("create Tailwind stylesheet directory: %w", err)
	}
	if err := os.WriteFile(inputPath, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write Tailwind stylesheet: %w", err)
	}
	return nil
}

func tailwindRunCommand(packageManager string, root string, inputPath string, outputPath string, watch bool) (string, []string) {
	args := []string{
		"-i", slashRel(root, inputPath),
		"-o", slashRel(root, outputPath),
	}
	if watch {
		args = append(args, "--watch")
	}

	switch packageManager {
	case "pnpm":
		return "pnpm", append([]string{"exec", "tailwindcss"}, args...)
	case "yarn":
		return "yarn", append([]string{"tailwindcss"}, args...)
	default:
		return "npx", append([]string{"@tailwindcss/cli"}, args...)
	}
}

func ensurePackageDevDependencies(path string, packages []string) (bool, error) {
	document, err := readPackageJSON(path)
	if err != nil {
		return false, err
	}

	changed := false
	if _, ok := document["private"]; !ok {
		document["private"] = true
		changed = true
	}

	dependencies, err := objectField(document, "dependencies")
	if err != nil {
		return false, err
	}
	devDependencies, err := objectField(document, "devDependencies")
	if err != nil {
		return false, err
	}

	for _, name := range packages {
		if name == "" {
			continue
		}
		if dependencies != nil {
			if _, ok := dependencies[name]; ok {
				continue
			}
		}
		if _, ok := devDependencies[name]; ok {
			continue
		}
		if devDependencies == nil {
			devDependencies = map[string]any{}
			document["devDependencies"] = devDependencies
			changed = true
		}
		devDependencies[name] = "latest"
		changed = true
	}

	if !changed {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create package.json directory: %w", err)
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal package.json: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return false, fmt.Errorf("write package.json: %w", err)
	}
	return true, nil
}

func readPackageJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}
	return document, nil
}

func objectField(document map[string]any, name string) (map[string]any, error) {
	value, ok := document[name]
	if !ok {
		return nil, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("package.json field %q must be an object", name)
	}
	return object, nil
}

func findAppRoot(start string) (string, error) {
	current := start
	for {
		candidate := filepath.Join(current, "go.mod")
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return current, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect %s: %w", candidate, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("go.mod not found")
		}
		current = parent
	}
}

func detectPackageManager(dir string) (string, []string) {
	if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
		return "pnpm", []string{"install"}
	}
	if fileExists(filepath.Join(dir, "yarn.lock")) {
		return "yarn", []string{"install"}
	}
	return "npm", []string{"install"}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func resolvePath(root, value string) string {
	value = strings.TrimSpace(value)
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(root, filepath.FromSlash(value))
}

func slashRel(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func samePath(a string, b string) bool {
	aAbs, aErr := filepath.Abs(a)
	bAbs, bErr := filepath.Abs(b)
	if aErr == nil && bErr == nil {
		return filepath.Clean(aAbs) == filepath.Clean(bAbs)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
