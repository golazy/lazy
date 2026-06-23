package upgrade

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golazy.dev/lazy/commands"
	"golazy.dev/lazy/commands/miseconfig"
	"golazy.dev/lazytui/progress"
)

var releaseVersions = []string{
	"v0.1.0",
	"v0.1.1",
	"v0.1.2",
	"v0.1.3",
	"v0.1.4",
	"v0.1.5",
	"v0.1.6",
	"v0.1.7",
	"v0.1.8",
	"v0.1.9",
	"v0.1.10",
	"v0.1.11",
	"v0.1.12",
	"v0.1.13",
	"v0.1.14",
}

const firstUpgradeAwareVersion = "v0.1.10"

type Command struct {
	Dir          string
	Target       string
	Force        string
	From         string
	To           string
	InternalStep bool
	DryRun       bool
	SkipCommands bool
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	Runner       commands.Runner
}

func (c Command) Execute() (int, error) {
	dir, err := c.workingDir()
	if err != nil {
		return 1, err
	}
	if c.InternalStep {
		if strings.TrimSpace(c.From) == "" || strings.TrimSpace(c.To) == "" {
			return 1, fmt.Errorf("internal upgrade step requires --from and --to")
		}
		return c.runStepProgress(dir, c.From, c.To, false)
	}

	if strings.TrimSpace(c.Force) != "" {
		if strings.TrimSpace(c.Target) != "" {
			return 1, fmt.Errorf("--force and --target cannot be used together")
		}
		step, err := forcedStep(c.Force)
		if err != nil {
			return 1, err
		}
		if !hasBuiltInStep(step.From, step.To) {
			return 1, fmt.Errorf("upgrade from %s to %s is not implemented; use the versioned upgrade guide", step.From, step.To)
		}
		return c.runStepProgress(dir, step.From, step.To, true)
	}

	module, err := readModule(filepath.Join(dir, "go.mod"))
	if err != nil {
		return 1, err
	}
	steps, err := planUpgrade(module.GoLazyVersion, c.Target)
	if err != nil {
		return 1, err
	}
	if len(steps) == 0 {
		if err := c.runProgress(progress.Tasks{
			progress.UITask("Check mise Go tool", func(ui *progress.UI) error {
				return c.checkMiseGoTool(dir, ui)
			}),
		}); err != nil {
			return 1, err
		}
		fmt.Fprintf(c.stdout(), "lazy: %s is already at %s\n", module.Path, module.GoLazyVersion)
		return 0, nil
	}
	for _, step := range steps {
		if !hasBuiltInStep(step.From, step.To) {
			return 1, fmt.Errorf("upgrade from %s to %s is not implemented; use the versioned upgrade guide", step.From, step.To)
		}
	}

	tasks := make(progress.Tasks, 0, len(steps)+1)
	for _, step := range steps {
		if step.BootstrapFromOlder {
			fmt.Fprintf(c.stdout(), "lazy: app uses an older pre-upgrade version; starting automated migrations at %s -> %s\n", step.From, step.To)
		}
		step := step
		tasks = append(tasks, progress.UITask(upgradeTaskName(step.From, step.To), func(ui *progress.UI) error {
			if c.DryRun {
				return ui.Takeover(func(_ io.Reader, stdout io.Writer, stderr io.Writer) error {
					_, err := c.runStepWithStreams(dir, step.From, step.To, step.BootstrapFromOlder, stdout, stderr, nil)
					return err
				})
			}
			_, err := c.runStep(dir, step.From, step.To, step.BootstrapFromOlder, ui)
			return err
		}))
	}
	tasks = append(tasks, progress.UITask("Check mise Go tool", func(ui *progress.UI) error {
		return c.checkMiseGoTool(dir, ui)
	}))
	if err := c.runProgress(tasks); err != nil {
		return 1, err
	}
	return 0, nil
}

func (c Command) workingDir() (string, error) {
	if strings.TrimSpace(c.Dir) != "" {
		return c.Dir, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return dir, nil
}

func (c Command) stdout() io.Writer {
	if c.Stdout == nil {
		return io.Discard
	}
	return c.Stdout
}

func (c Command) stderr() io.Writer {
	if c.Stderr == nil {
		return io.Discard
	}
	return c.Stderr
}

func (c Command) runProgress(tasks progress.Tasks) error {
	return progress.Run(tasks, c.Stdin, c.stdout(), c.stderr())
}

func (c Command) runner() commands.Runner {
	if c.Runner != nil {
		return c.Runner
	}
	return commands.Exec
}

func (c Command) checkMiseGoTool(dir string, ui *progress.UI) error {
	run := func(stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
		return (miseconfig.GoToolCheck{
			Dir:    dir,
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
			DryRun: c.DryRun,
		}).Execute()
	}
	if ui == nil {
		return run(c.Stdin, c.stdout(), c.stderr())
	}
	found, err := miseGoToolFound(dir)
	if err != nil {
		return err
	}
	if found {
		return ui.Takeover(run)
	}
	return ui.Run(run)
}

func miseGoToolFound(dir string) (bool, error) {
	path := filepath.Join(dir, "mise.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read mise.toml: %w", err)
	}
	_, found := miseconfig.RemoveGoTool(data)
	return found, nil
}

func upgradeTaskName(from string, to string) string {
	return fmt.Sprintf("Upgrade %s -> %s", from, to)
}

func (c Command) runStepProgress(dir string, from string, to string, force bool) (int, error) {
	err := c.runProgress(progress.Tasks{
		progress.UITask(upgradeTaskName(from, to), func(ui *progress.UI) error {
			if c.DryRun {
				return ui.Takeover(func(_ io.Reader, stdout io.Writer, stderr io.Writer) error {
					_, err := c.runStepWithStreams(dir, from, to, force, stdout, stderr, nil)
					return err
				})
			}
			_, err := c.runStep(dir, from, to, force, ui)
			return err
		}),
	})
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func (c Command) runStep(dir string, from string, to string, force bool, ui *progress.UI) (int, error) {
	stdout := c.stdout()
	stderr := c.stderr()
	if ui != nil {
		stdout = ui.Stdout()
		stderr = ui.Stderr()
	}
	return c.runStepWithStreams(dir, from, to, force, stdout, stderr, ui)
}

func (c Command) runStepWithStreams(dir string, from string, to string, force bool, stdout io.Writer, stderr io.Writer, ui *progress.UI) (int, error) {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	module, err := readModule(filepath.Join(dir, "go.mod"))
	if err != nil {
		return 1, err
	}
	if module.GoLazyVersion != from && !c.DryRun && !force {
		return 1, fmt.Errorf("go.mod requires golazy.dev %s, want %s before upgrading to %s", module.GoLazyVersion, from, to)
	}
	executor := stepExecutor{
		dir:                dir,
		modulePath:         module.Path,
		from:               from,
		to:                 to,
		bootstrapFromOlder: force,
		dryRun:             c.DryRun,
		skipCommands:       c.SkipCommands,
		stdout:             stdout,
		stderr:             stderr,
		runner:             c.runner(),
		customRunner:       c.Runner != nil,
		ui:                 ui,
	}

	switch {
	case from == "v0.1.10" && to == "v0.1.11":
		err = executor.upgradeTo011()
	case from == "v0.1.11" && to == "v0.1.12":
		err = executor.upgradeTo012()
	case from == "v0.1.12" && to == "v0.1.13":
		err = executor.upgradeTo013()
	default:
		err = fmt.Errorf("upgrade from %s to %s is not implemented; use the versioned upgrade guide", from, to)
	}
	if err != nil {
		return 1, err
	}
	if err := executor.updateGoMod(); err != nil {
		return 1, err
	}
	if err := executor.runFollowups(); err != nil {
		return 1, err
	}
	return 0, nil
}

type moduleInfo struct {
	Path          string
	GoLazyVersion string
}

func readModule(filename string) (moduleInfo, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return moduleInfo{}, fmt.Errorf("go.mod not found")
		}
		return moduleInfo{}, fmt.Errorf("read go.mod: %w", err)
	}
	path := modfile.ModulePath(data)
	if path == "" {
		return moduleInfo{}, fmt.Errorf("go.mod does not declare a module")
	}
	file, err := modfile.Parse(filename, data, nil)
	if err != nil {
		return moduleInfo{}, fmt.Errorf("parse go.mod: %w", err)
	}
	for _, require := range file.Require {
		if require.Mod.Path == "golazy.dev" {
			return moduleInfo{Path: path, GoLazyVersion: require.Mod.Version}, nil
		}
	}
	return moduleInfo{}, fmt.Errorf("go.mod does not require golazy.dev")
}

type upgradeStep struct {
	From               string
	To                 string
	BootstrapFromOlder bool
}

func planUpgrade(current string, target string) ([]upgradeStep, error) {
	currentIndex := slices.Index(releaseVersions, current)
	if currentIndex == -1 {
		return nil, fmt.Errorf("unsupported current GoLazy version %s", current)
	}
	firstAwareIndex := slices.Index(releaseVersions, firstUpgradeAwareVersion)
	if firstAwareIndex == -1 {
		return nil, fmt.Errorf("first upgrade-aware GoLazy version %s is not in release list", firstUpgradeAwareVersion)
	}
	startsBeforeAware := currentIndex < firstAwareIndex

	target = strings.TrimSpace(target)
	if target == "" {
		if startsBeforeAware {
			if firstAwareIndex == len(releaseVersions)-1 {
				return nil, nil
			}
			target = releaseVersions[firstAwareIndex+1]
		} else if currentIndex == len(releaseVersions)-1 {
			return nil, nil
		} else {
			target = releaseVersions[currentIndex+1]
		}
	}
	targetIndex := slices.Index(releaseVersions, target)
	if targetIndex == -1 {
		return nil, fmt.Errorf("unknown target GoLazy version %s", target)
	}
	if startsBeforeAware && targetIndex <= firstAwareIndex {
		return nil, fmt.Errorf("target GoLazy version %s is before the first automated upgrade path %s -> %s; use the versioned upgrade guide", target, firstUpgradeAwareVersion, releaseVersions[firstAwareIndex+1])
	}
	if !startsBeforeAware && targetIndex < currentIndex {
		return nil, fmt.Errorf("target GoLazy version %s is older than current version %s", target, current)
	}
	if targetIndex == currentIndex {
		return nil, nil
	}

	stepStartIndex := currentIndex + 1
	if startsBeforeAware {
		stepStartIndex = firstAwareIndex + 1
	}
	var steps []upgradeStep
	for index := stepStartIndex; index <= targetIndex; index++ {
		steps = append(steps, upgradeStep{
			From:               releaseVersions[index-1],
			To:                 releaseVersions[index],
			BootstrapFromOlder: startsBeforeAware && index == firstAwareIndex+1,
		})
	}
	return steps, nil
}

func forcedStep(version string) (upgradeStep, error) {
	version = strings.TrimSpace(version)
	index := slices.Index(releaseVersions, version)
	if index == -1 {
		return upgradeStep{}, fmt.Errorf("unknown forced GoLazy version %s", version)
	}
	if index == len(releaseVersions)-1 {
		return upgradeStep{}, fmt.Errorf("GoLazy version %s has no later upgrade step", version)
	}
	return upgradeStep{
		From: releaseVersions[index],
		To:   releaseVersions[index+1],
	}, nil
}

func hasBuiltInStep(from string, to string) bool {
	switch {
	case from == "v0.1.10" && to == "v0.1.11":
		return true
	case from == "v0.1.11" && to == "v0.1.12":
		return true
	case from == "v0.1.12" && to == "v0.1.13":
		return true
	default:
		return false
	}
}

type stepExecutor struct {
	dir                string
	modulePath         string
	from               string
	to                 string
	bootstrapFromOlder bool
	dryRun             bool
	skipCommands       bool
	stdout             io.Writer
	stderr             io.Writer
	runner             commands.Runner
	customRunner       bool
	ui                 *progress.UI
}

func (e stepExecutor) upgradeTo011() error {
	if err := e.replaceFileIfHash("mise.toml", v010MiseToml, v011MiseToml, 0o644); err != nil {
		return err
	}
	if err := e.addFile(".mise/tasks/dev", v011DevTask, 0o755); err != nil {
		return err
	}
	return e.addFile(".mise/tasks/test", v011TestTask, 0o755)
}

func (e stepExecutor) upgradeTo012() error {
	if err := e.moveServices(); err != nil {
		return err
	}
	return e.rewriteServiceImports()
}

func (e stepExecutor) upgradeTo013() error {
	return nil
}

func (e stepExecutor) updateGoMod() error {
	path := filepath.Join(e.dir, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read go.mod: %w", err)
	}
	file, err := modfile.Parse(path, data, nil)
	if err != nil {
		return fmt.Errorf("parse go.mod: %w", err)
	}
	if err := file.AddRequire("golazy.dev", e.to); err != nil {
		return fmt.Errorf("update golazy.dev requirement: %w", err)
	}
	formatted, err := file.Format()
	if err != nil {
		return fmt.Errorf("format go.mod: %w", err)
	}
	if bytes.Equal(data, formatted) {
		fmt.Fprintf(e.stdout, "  go.mod already requires golazy.dev %s\n", e.to)
		return nil
	}
	if e.dryRun {
		fmt.Fprintf(e.stdout, "  would update go.mod to golazy.dev %s\n", e.to)
		return nil
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return fmt.Errorf("write go.mod: %w", err)
	}
	fmt.Fprintf(e.stdout, "  updated go.mod to golazy.dev %s\n", e.to)
	return nil
}

func (e stepExecutor) replaceFileIfHash(relative string, previous string, target string, mode os.FileMode) error {
	path := filepath.Join(e.dir, relative)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && e.bootstrapFromOlder {
			return e.addFile(relative, target, mode)
		}
		return fmt.Errorf("read %s: %w", relative, err)
	}
	targetData := []byte(target)
	switch {
	case bytes.Equal(data, targetData):
		fmt.Fprintf(e.stdout, "  %s already matches %s\n", relative, e.to)
		return nil
	case bytes.Equal(data, []byte(previous)):
		if e.dryRun {
			fmt.Fprintf(e.stdout, "  would update %s\n", relative)
			return nil
		}
		if err := os.WriteFile(path, targetData, mode); err != nil {
			return fmt.Errorf("write %s: %w", relative, err)
		}
		fmt.Fprintf(e.stdout, "  updated %s\n", relative)
		return nil
	default:
		return e.fileConflict(relative, data, targetData)
	}
}

func (e stepExecutor) addFile(relative string, target string, mode os.FileMode) error {
	path := filepath.Join(e.dir, relative)
	targetData := []byte(target)
	data, err := os.ReadFile(path)
	if err == nil {
		if bytes.Equal(data, targetData) {
			fmt.Fprintf(e.stdout, "  %s already exists\n", relative)
			return nil
		}
		return e.fileConflict(relative, data, targetData)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", relative, err)
	}
	if e.dryRun {
		fmt.Fprintf(e.stdout, "  would add %s\n", relative)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(relative), err)
	}
	if err := os.WriteFile(path, targetData, mode); err != nil {
		return fmt.Errorf("write %s: %w", relative, err)
	}
	fmt.Fprintf(e.stdout, "  added %s\n", relative)
	return nil
}

func (e stepExecutor) fileConflict(relative string, current []byte, proposed []byte) error {
	diff := fullFileDiff(relative, current, proposed)
	if e.ui != nil {
		if err := e.ui.Takeover(func(_ io.Reader, _ io.Writer, stderr io.Writer) error {
			fmt.Fprint(stderr, diff)
			return nil
		}); err != nil {
			return err
		}
	} else {
		fmt.Fprint(e.stderr, diff)
	}
	if !e.dryRun {
		proposedPath := filepath.Join(e.dir, ".golazy", "upgrade", "conflicts", e.to, relative)
		if err := os.MkdirAll(filepath.Dir(proposedPath), 0o755); err != nil {
			return fmt.Errorf("create conflict directory for %s: %w", relative, err)
		}
		if err := os.WriteFile(proposedPath, proposed, 0o644); err != nil {
			return fmt.Errorf("write proposed %s: %w", relative, err)
		}
	}
	return fmt.Errorf("upgrade conflict in %s; review the diff, edit the file, and rerun lazy upgrade", relative)
}

func fullFileDiff(relative string, current []byte, proposed []byte) string {
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n", relative)
	fmt.Fprintf(&out, "+++ proposed/%s\n", relative)
	fmt.Fprintln(&out, "@@")
	for _, line := range splitLines(string(current)) {
		fmt.Fprintf(&out, "-%s\n", line)
	}
	for _, line := range splitLines(string(proposed)) {
		fmt.Fprintf(&out, "+%s\n", line)
	}
	return out.String()
}

func splitLines(value string) []string {
	value = strings.TrimSuffix(value, "\n")
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}

func (e stepExecutor) moveServices() error {
	source := filepath.Join(e.dir, "app", "services")
	destination := filepath.Join(e.dir, "services")
	sourceInfo, sourceErr := os.Stat(source)
	destinationInfo, destinationErr := os.Stat(destination)
	switch {
	case sourceErr == nil && !sourceInfo.IsDir():
		return fmt.Errorf("app/services exists but is not a directory")
	case destinationErr == nil && !destinationInfo.IsDir():
		return fmt.Errorf("services exists but is not a directory")
	case sourceErr == nil && destinationErr == nil:
		return fmt.Errorf("cannot move app/services to services because both directories exist")
	case errors.Is(sourceErr, os.ErrNotExist) && destinationErr == nil:
		fmt.Fprintln(e.stdout, "  services already moved")
		return nil
	case errors.Is(sourceErr, os.ErrNotExist):
		fmt.Fprintln(e.stdout, "  app/services not present; skipping service move")
		return nil
	case sourceErr != nil:
		return fmt.Errorf("inspect app/services: %w", sourceErr)
	case destinationErr != nil && !errors.Is(destinationErr, os.ErrNotExist):
		return fmt.Errorf("inspect services: %w", destinationErr)
	}

	if e.dryRun {
		fmt.Fprintln(e.stdout, "  would move app/services to services")
		return nil
	}
	if err := os.Rename(source, destination); err != nil {
		return fmt.Errorf("move app/services to services: %w", err)
	}
	fmt.Fprintln(e.stdout, "  moved app/services to services")
	return nil
}

func (e stepExecutor) rewriteServiceImports() error {
	oldPrefix := e.modulePath + "/app/services"
	newPrefix := e.modulePath + "/services"
	var rewritten []string
	if err := filepath.WalkDir(e.dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".golazy", "node_modules":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		changed, err := e.rewriteServiceImportsInFile(path, oldPrefix, newPrefix)
		if err != nil {
			return err
		}
		if changed {
			relative, relErr := filepath.Rel(e.dir, path)
			if relErr != nil {
				return relErr
			}
			rewritten = append(rewritten, relative)
		}
		return nil
	}); err != nil {
		return err
	}
	if len(rewritten) == 0 {
		fmt.Fprintln(e.stdout, "  no app/services imports to rewrite")
		return nil
	}
	slices.Sort(rewritten)
	for _, path := range rewritten {
		if e.dryRun {
			fmt.Fprintf(e.stdout, "  would rewrite imports in %s\n", path)
		} else {
			fmt.Fprintf(e.stdout, "  rewrote imports in %s\n", path)
		}
	}
	return nil
}

func (e stepExecutor) rewriteServiceImportsInFile(path string, oldPrefix string, newPrefix string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, path, data, 0)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	changed := false
	for _, imported := range file.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			continue
		}
		if importPath == oldPrefix || strings.HasPrefix(importPath, oldPrefix+"/") {
			imported.Path.Value = strconv.Quote(newPrefix + strings.TrimPrefix(importPath, oldPrefix))
			changed = true
		}
	}
	if !changed || e.dryRun {
		return changed, nil
	}
	var formatted bytes.Buffer
	if err := format.Node(&formatted, files, file); err != nil {
		return false, fmt.Errorf("format %s: %w", path, err)
	}
	if err := os.WriteFile(path, formatted.Bytes(), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

func (e stepExecutor) runFollowups() error {
	calls := []struct {
		command string
		args    []string
	}{
		{command: "go", args: []string{"mod", "tidy"}},
		{command: "go", args: []string{"test", "./..."}},
		{command: "go", args: []string{"vet", "./..."}},
	}
	for _, call := range calls {
		displayCommand := call.command
		displayArgs := call.args
		if e.dryRun || e.skipCommands {
			fmt.Fprintf(e.stdout, "  would run %s %s\n", displayCommand, strings.Join(displayArgs, " "))
			continue
		}
		fmt.Fprintf(e.stdout, "  running %s %s\n", displayCommand, strings.Join(displayArgs, " "))
		if err := e.runner(call.command, call.args, commands.Options{
			Dir:    e.dir,
			Stdout: e.stdout,
			Stderr: e.stderr,
		}); err != nil {
			return fmt.Errorf("%s %s: %w", displayCommand, strings.Join(displayArgs, " "), err)
		}
	}
	return nil
}

const v010MiseToml = `[tools]
go = "1.26.0"
"aqua:FiloSottile/age" = "latest"
"aqua:getsops/sops" = "latest"
"aqua:jdx/usage" = "latest"

[env]
_.file = ".secrets/development.env"

[tasks.dev]
description = "Run the GoLazy development server with development secrets."
run = "lazy"

[tasks.test]
description = "Run the sample app tests."
run = "go test ./..."
`

const v011MiseToml = `[tools]
node = "24"
"aqua:FiloSottile/age" = "latest"
"aqua:getsops/sops" = "latest"
"aqua:jdx/usage" = "latest"

[env]
_.file = ".secrets/development.env"
`

const v011DevTask = `#!/usr/bin/env bash
#MISE description="Run the GoLazy development server with development secrets"
set -euo pipefail

lazy
`

const v011TestTask = `#!/usr/bin/env bash
#MISE description="Run the sample app tests"
set -euo pipefail

go test ./...
`
