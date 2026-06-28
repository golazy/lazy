package upgradeservice

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golazy.dev/lazy/services/execservice"
	"golazy.dev/lazy/services/lazycodeservice"
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
	"v0.1.15",
	"v0.1.16",
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
	Runner       execservice.Runner
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
			progress.UITask("Check mise tools", func(ui *progress.UI) error {
				return c.applyCurrentMiseManifest(dir, module.GoLazyVersion, ui)
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
	tasks = append(tasks, progress.UITask("Check mise tools", func(ui *progress.UI) error {
		return c.applyCurrentMiseManifest(dir, steps[len(steps)-1].To, ui)
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

func (c Command) runner() execservice.Runner {
	if c.Runner != nil {
		return c.Runner
	}
	return execservice.Exec
}

func (c Command) applyCurrentMiseManifest(dir string, version string, ui *progress.UI) error {
	stdout := c.stdout()
	stderr := c.stderr()
	if ui != nil {
		stdout = ui.Stdout()
		stderr = ui.Stderr()
	}
	return (stepExecutor{
		dir:    dir,
		to:     version,
		dryRun: c.DryRun,
		stdout: stdout,
		stderr: stderr,
	}).applyMiseManifest(currentMiseCleanupManifest(version))
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
		stdin:              c.Stdin,
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
	case from == "v0.1.14" && to == "v0.1.15":
		err = executor.upgradeTo015()
	case from == "v0.1.15" && to == "v0.1.16":
		err = executor.upgradeTo016()
	default:
		err = fmt.Errorf("upgrade from %s to %s is not implemented; use the versioned upgrade guide", from, to)
	}
	if err != nil {
		return 1, err
	}
	if err := executor.applyGoModManifest(goModManifestFor(from, to)); err != nil {
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
	case from == "v0.1.14" && to == "v0.1.15":
		return true
	case from == "v0.1.15" && to == "v0.1.16":
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
	stdin              io.Reader
	stdout             io.Writer
	stderr             io.Writer
	runner             execservice.Runner
	customRunner       bool
	ui                 *progress.UI
}

func (e stepExecutor) upgradeTo011() error {
	if err := e.applyFileManifest(upgradeTo011Manifest()); err != nil {
		return err
	}
	return e.applyMiseManifest(upgradeTo011MiseManifest())
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

func (e stepExecutor) upgradeTo015() error {
	if err := e.migrateContextToDependencies(); err != nil {
		return err
	}
	if err := e.migrateSEOToFunction(); err != nil {
		return err
	}
	return nil
}

func (e stepExecutor) upgradeTo016() error {
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

func (e stepExecutor) migrateContextToDependencies() error {
	if err := e.rewriteAppConfigDependencies(); err != nil {
		return err
	}
	return e.rewriteContextInitializer()
}

func (e stepExecutor) rewriteAppConfigDependencies() error {
	path := filepath.Join(e.dir, "init", "app.go")
	changed, err := lazycodeservice.RewriteFile(path, e.dryRun, func(_ *token.FileSet, file *ast.File) (bool, error) {
		changed := false
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok || !isLazyappConfig(literal.Type) {
				return true
			}
			for _, element := range literal.Elts {
				keyValue, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := keyValue.Key.(*ast.Ident)
				if ok && key.Name == "Context" {
					key.Name = "Dependencies"
					keyValue.Value = renameContextInitializerValue(keyValue.Value)
					changed = true
				}
			}
			return true
		})
		return changed, nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(e.stdout, "  init/app.go not present; skipping lazyapp.Config dependency field rewrite")
			return nil
		}
		return err
	}
	switch {
	case changed && e.dryRun:
		fmt.Fprintln(e.stdout, "  would rewrite lazyapp.Config Context field to Dependencies")
	case changed:
		fmt.Fprintln(e.stdout, "  rewrote lazyapp.Config Context field to Dependencies")
	default:
		fmt.Fprintln(e.stdout, "  lazyapp.Config already uses Dependencies")
	}
	return nil
}

func renameContextInitializerValue(expr ast.Expr) ast.Expr {
	switch value := expr.(type) {
	case *ast.Ident:
		if value.Name == "Context" {
			value.Name = "Dependencies"
		}
	case *ast.CallExpr:
		if ident, ok := value.Fun.(*ast.Ident); ok && ident.Name == "Context" {
			ident.Name = "Dependencies"
		}
	}
	return expr
}

func isLazyappConfig(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Config" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == "lazyapp"
}

func (e stepExecutor) rewriteContextInitializer() error {
	source := filepath.Join(e.dir, "init", "context.go")
	target := filepath.Join(e.dir, "init", "dependencies.go")
	if _, err := os.Stat(source); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(e.stdout, "  init/context.go not present; skipping dependency initializer rename")
			return nil
		}
		return fmt.Errorf("inspect init/context.go: %w", err)
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("cannot migrate init/context.go because init/dependencies.go already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect init/dependencies.go: %w", err)
	}

	rewritePath := source
	if !e.dryRun {
		if err := os.Rename(source, target); err != nil {
			return fmt.Errorf("rename init/context.go to init/dependencies.go: %w", err)
		}
		rewritePath = target
	}
	changed, err := lazycodeservice.RewriteFile(rewritePath, e.dryRun, rewriteContextInitializerFile)
	if err != nil {
		return err
	}
	switch {
	case e.dryRun:
		if changed {
			fmt.Fprintln(e.stdout, "  would rename init/context.go to init/dependencies.go and rewrite Context as Dependencies")
		} else {
			fmt.Fprintln(e.stdout, "  would rename init/context.go to init/dependencies.go")
		}
	case changed:
		fmt.Fprintln(e.stdout, "  renamed init/context.go to init/dependencies.go and rewrote Context as Dependencies")
	default:
		fmt.Fprintln(e.stdout, "  renamed init/context.go to init/dependencies.go")
	}
	return nil
}

func rewriteContextInitializerFile(_ *token.FileSet, file *ast.File) (bool, error) {
	changed := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Context" || fn.Body == nil {
			continue
		}
		contextName := contextParameterName(fn)
		if contextName == "" || contextName == "_" {
			contextName = "ctx"
		}
		fn.Name.Name = "Dependencies"
		fn.Type.Params = &ast.FieldList{List: []*ast.Field{{
			Names: []*ast.Ident{ast.NewIdent("deps")},
			Type: &ast.StarExpr{X: &ast.SelectorExpr{
				X:   ast.NewIdent("lazydeps"),
				Sel: ast.NewIdent("Scope"),
			}},
		}}}
		fn.Type.Results = &ast.FieldList{List: []*ast.Field{{
			Type: ast.NewIdent("error"),
		}}}
		fn.Body.List = append([]ast.Stmt{contextAssignStmt(contextName)}, fn.Body.List...)
		rewriteReturnsInBlock(fn.Body)
		changed = true
		break
	}
	if changed {
		if lazycodeservice.EnsureImport(file, "golazy.dev/lazydeps") {
			changed = true
		}
		if !lazycodeservice.UsesSelector(file, "context") {
			lazycodeservice.RemoveImport(file, "context")
		}
	}
	return changed, nil
}

func contextParameterName(fn *ast.FuncDecl) string {
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 || len(fn.Type.Params.List[0].Names) == 0 {
		return ""
	}
	return fn.Type.Params.List[0].Names[0].Name
}

func contextAssignStmt(name string) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(name)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("deps"),
			Sel: ast.NewIdent("Context"),
		}}},
	}
}

func rewriteReturnsInBlock(block *ast.BlockStmt) {
	var statements []ast.Stmt
	for _, statement := range block.List {
		switch stmt := statement.(type) {
		case *ast.ReturnStmt:
			statements = append(statements, dependencyReturnStatements(stmt)...)
		case *ast.IfStmt:
			rewriteReturnsInBlock(stmt.Body)
			rewriteReturnsInElse(stmt.Else)
			statements = append(statements, stmt)
		case *ast.ForStmt:
			rewriteReturnsInBlock(stmt.Body)
			statements = append(statements, stmt)
		case *ast.RangeStmt:
			rewriteReturnsInBlock(stmt.Body)
			statements = append(statements, stmt)
		case *ast.SwitchStmt:
			rewriteReturnsInBlock(stmt.Body)
			statements = append(statements, stmt)
		case *ast.TypeSwitchStmt:
			rewriteReturnsInBlock(stmt.Body)
			statements = append(statements, stmt)
		case *ast.SelectStmt:
			rewriteReturnsInBlock(stmt.Body)
			statements = append(statements, stmt)
		default:
			statements = append(statements, statement)
		}
	}
	block.List = statements
}

func rewriteReturnsInElse(statement ast.Stmt) {
	switch stmt := statement.(type) {
	case *ast.BlockStmt:
		rewriteReturnsInBlock(stmt)
	case *ast.IfStmt:
		rewriteReturnsInBlock(stmt.Body)
		rewriteReturnsInElse(stmt.Else)
	}
}

func dependencyReturnStatements(stmt *ast.ReturnStmt) []ast.Stmt {
	switch len(stmt.Results) {
	case 0:
		return []ast.Stmt{returnNilStmt()}
	case 1:
		if isNilIdent(stmt.Results[0]) {
			return []ast.Stmt{returnNilStmt()}
		}
		return []ast.Stmt{setContextStmt(stmt.Results[0]), returnNilStmt()}
	default:
		if isNilIdent(stmt.Results[1]) {
			return []ast.Stmt{setContextStmt(stmt.Results[0]), returnNilStmt()}
		}
		return []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{stmt.Results[1]}}}
	}
}

func setContextStmt(expr ast.Expr) ast.Stmt {
	return &ast.ExprStmt{X: &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("deps"),
			Sel: ast.NewIdent("SetContext"),
		},
		Args: []ast.Expr{expr},
	}}
}

func returnNilStmt() ast.Stmt {
	return &ast.ReturnStmt{Results: []ast.Expr{ast.NewIdent("nil")}}
}

func isNilIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

type seoInitializerMigration struct {
	PackageName       string
	OptionsExpression string
	OptionsPackage    string
	Imports           []goImport
}

type goImport struct {
	Name string
	Path string
}

func (e stepExecutor) migrateSEOToFunction() error {
	migration, changed, err := e.rewriteAppConfigSEO()
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintln(e.stdout, "  lazyapp.Config already uses SEO initializer")
		return nil
	}
	if e.dryRun {
		fmt.Fprintln(e.stdout, "  would add init/seo.go for lazyapp.Config SEO defaults")
		return nil
	}
	if err := e.addFile("init/seo.go", migration.File(), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(e.stdout, "  moved lazyapp.Config SEO defaults to init/seo.go")
	return nil
}

func (e stepExecutor) rewriteAppConfigSEO() (seoInitializerMigration, bool, error) {
	path := filepath.Join(e.dir, "init", "app.go")
	var migration seoInitializerMigration
	changed, err := lazycodeservice.RewriteFile(path, e.dryRun, func(fileSet *token.FileSet, file *ast.File) (bool, error) {
		migration.PackageName = file.Name.Name
		imports := importsByName(file)
		changed := false
		var rewriteErr error
		var movedSelectorNames []string
		ast.Inspect(file, func(node ast.Node) bool {
			if changed || rewriteErr != nil {
				return false
			}
			literal, ok := node.(*ast.CompositeLit)
			if !ok || !isLazyappConfig(literal.Type) {
				return true
			}
			for _, element := range literal.Elts {
				keyValue, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := keyValue.Key.(*ast.Ident)
				if !ok || key.Name != "SEO" {
					continue
				}
				optionsPackage, ok := seoOptionsPackage(keyValue.Value)
				if !ok {
					continue
				}
				if !e.dryRun {
					if _, err := os.Stat(filepath.Join(e.dir, "init", "seo.go")); err == nil {
						rewriteErr = fmt.Errorf("cannot migrate lazyapp.Config SEO because init/seo.go already exists")
						return false
					} else if !errors.Is(err, os.ErrNotExist) {
						rewriteErr = fmt.Errorf("inspect init/seo.go: %w", err)
						return false
					}
				}
				expression, err := goNodeString(fileSet, keyValue.Value)
				if err != nil {
					rewriteErr = err
					return false
				}
				selectorNames := selectorPackageNames(keyValue.Value)
				migration = seoInitializerMigration{
					PackageName:       file.Name.Name,
					OptionsExpression: expression,
					OptionsPackage:    optionsPackage,
					Imports:           seoInitializerImports(imports, optionsPackage, selectorNames),
				}
				movedSelectorNames = selectorNames
				keyValue.Value = ast.NewIdent("SEO")
				changed = true
				return false
			}
			return true
		})
		if rewriteErr != nil {
			return false, rewriteErr
		}
		if changed {
			for _, name := range movedSelectorNames {
				imported, ok := imports[name]
				if !ok || lazycodeservice.UsesSelector(file, name) {
					continue
				}
				lazycodeservice.RemoveImport(file, imported.Path)
			}
		}
		return changed, nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(e.stdout, "  init/app.go not present; skipping lazyapp.Config SEO rewrite")
			return migration, false, nil
		}
		return migration, false, err
	}
	switch {
	case changed && e.dryRun:
		fmt.Fprintln(e.stdout, "  would rewrite lazyapp.Config SEO field to SEO initializer")
	case changed:
		fmt.Fprintln(e.stdout, "  rewrote lazyapp.Config SEO field to SEO initializer")
	}
	return migration, changed, nil
}

func seoOptionsPackage(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.CompositeLit)
	if !ok {
		return "", false
	}
	array, ok := literal.Type.(*ast.ArrayType)
	if !ok {
		return "", false
	}
	selector, ok := array.Elt.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Option" {
		return "", false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok || ident.Name == "" {
		return "", false
	}
	return ident.Name, true
}

func importsByName(file *ast.File) map[string]goImport {
	imports := make(map[string]goImport)
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path == "" {
			continue
		}
		name := importName(spec, path)
		if name == "" || name == "_" || name == "." {
			continue
		}
		imports[name] = goImport{Name: importAlias(spec), Path: path}
	}
	return imports
}

func importName(spec *ast.ImportSpec, path string) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	base := path
	if slash := strings.LastIndex(base, "/"); slash >= 0 {
		base = base[slash+1:]
	}
	return strings.TrimSpace(base)
}

func importAlias(spec *ast.ImportSpec) string {
	if spec.Name == nil {
		return ""
	}
	return spec.Name.Name
}

func selectorPackageNames(expr ast.Expr) []string {
	seen := make(map[string]struct{})
	var names []string
	ast.Inspect(expr, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || ident.Name == "" {
			return true
		}
		if _, ok := seen[ident.Name]; ok {
			return true
		}
		seen[ident.Name] = struct{}{}
		names = append(names, ident.Name)
		return true
	})
	slices.Sort(names)
	return names
}

func seoInitializerImports(imports map[string]goImport, optionsPackage string, selectorNames []string) []goImport {
	seen := make(map[string]struct{})
	var result []goImport
	add := func(imported goImport) {
		key := imported.Name + "\x00" + imported.Path
		if imported.Path == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, imported)
	}
	add(goImport{Path: "context"})
	if imported, ok := imports[optionsPackage]; ok {
		add(imported)
	} else {
		add(goImport{Path: "golazy.dev/lazyseo"})
	}
	for _, name := range selectorNames {
		imported, ok := imports[name]
		if !ok {
			continue
		}
		add(imported)
	}
	slices.SortFunc(result, func(a goImport, b goImport) int {
		if a.Path < b.Path {
			return -1
		}
		if a.Path > b.Path {
			return 1
		}
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return result
}

func goNodeString(fileSet *token.FileSet, node any) (string, error) {
	var out bytes.Buffer
	if err := printer.Fprint(&out, fileSet, node); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (m seoInitializerMigration) File() string {
	if m.PackageName == "" {
		m.PackageName = "appinit"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "package %s\n\n", m.PackageName)
	if len(m.Imports) > 0 {
		out.WriteString("import (\n")
		for _, imported := range m.Imports {
			if imported.Name != "" {
				fmt.Fprintf(&out, "\t%s %q\n", imported.Name, imported.Path)
			} else {
				fmt.Fprintf(&out, "\t%q\n", imported.Path)
			}
		}
		out.WriteString(")\n\n")
	}
	fmt.Fprintf(&out, "func SEO(ctx context.Context) []%s.Option {\n", m.OptionsPackage)
	fmt.Fprintf(&out, "\treturn %s\n", m.OptionsExpression)
	out.WriteString("}\n")
	formatted, err := format.Source([]byte(out.String()))
	if err != nil {
		return out.String()
	}
	return string(formatted)
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
	return lazycodeservice.RewriteFile(path, e.dryRun, func(_ *token.FileSet, file *ast.File) (bool, error) {
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
		return changed, nil
	})
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
		if err := e.runner(call.command, call.args, execservice.Options{
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
"aqua:FiloSottile/age" = "1.3.1"
"aqua:getsops/sops" = "3.13.1"
"aqua:jdx/usage" = "3.5.3"

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
