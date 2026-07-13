package addonservice

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golazy.dev/lazy/services/execservice"
	"golazy.dev/lazy/services/taskservice"
	"golazy.dev/lazycode"
)

// BuildHookPhase is one of the explicit points at which an add-on may run a
// declared mise task. Build hooks never contain commands themselves.
type BuildHookPhase string

const (
	BeforeBuild BuildHookPhase = "before-build"
	AfterBuild  BuildHookPhase = "after-build"
)

// BuildHook is one resolved build-time mise task from addons.toml.
type BuildHook struct {
	AddonID string
	Phase   BuildHookPhase
	Task    string
	Target  string
}

// BuildHookPlan is the immutable hook snapshot used for one application
// build. Before- and after-build phases therefore cannot observe different
// addons.toml contents if a task happens to rewrite the lock while it runs.
type BuildHookPlan struct {
	hooks []BuildHook
}

// BuildHookRunner reads the exact add-on lock and runs one phase. Runner is an
// injection seam for tests; production execution goes through
// taskservice.RunTaskContext and mise.
type BuildHookRunner struct {
	Root   string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Runner execservice.Runner
}

// Run executes all hooks in dependency-first order. Add-on IDs and task names
// are deterministic tie breakers.
func (runner BuildHookRunner) Run(phase BuildHookPhase) error {
	return runner.RunContext(context.Background(), phase)
}

// RunContext loads and runs a build-hook phase with cancellation.
func (runner BuildHookRunner) RunContext(ctx context.Context, phase BuildHookPhase) error {
	plan, err := LoadBuildHookPlan(runner.Root)
	if err != nil {
		return err
	}
	return plan.RunContext(ctx, phase, runner)
}

// LoadBuildHookPlan reads and orders the exact add-on lock once for a build.
func LoadBuildHookPlan(root string) (BuildHookPlan, error) {
	state, err := ReadState(root)
	if err != nil {
		return BuildHookPlan{}, err
	}
	hooks, err := allBuildHooks(state)
	if err != nil {
		return BuildHookPlan{}, err
	}
	if err := validateLockedBuildHooks(root, state, hooks); err != nil {
		return BuildHookPlan{}, err
	}
	return BuildHookPlan{hooks: hooks}, nil
}

// Run executes one phase from an already loaded build plan.
func (plan BuildHookPlan) Run(phase BuildHookPhase, runner BuildHookRunner) error {
	return plan.RunContext(context.Background(), phase, runner)
}

// RunContext executes one phase from an already loaded plan with
// cancellation propagated to the mise subprocess.
func (plan BuildHookPlan) RunContext(ctx context.Context, phase BuildHookPhase, runner BuildHookRunner) error {
	if !validBuildHookPhase(phase) {
		return fmt.Errorf("unknown add-on build hook phase %q", phase)
	}
	for _, hook := range plan.hooks {
		if hook.Phase != phase {
			continue
		}
		if runner.Stdout != nil {
			fmt.Fprintf(runner.Stdout, "lazy: add-on %s %s hook: mise run %s\n", hook.AddonID, hook.Phase, hook.Task)
		}
		if err := taskservice.RunTaskContext(ctx, runner.Runner, runner.Root, hook.Task, nil, runner.Stdin, runner.Stdout, runner.Stderr, false); err != nil {
			return fmt.Errorf("add-on %q %s hook %q: %w", hook.AddonID, hook.Phase, hook.Task, err)
		}
	}
	return nil
}

func validateBuildContributions(addonID string, build BuildContributions) error {
	declared := make(map[string]bool, len(build.Tasks))
	for _, task := range build.Tasks {
		if !validMiseTaskName(task) {
			return fmt.Errorf("add-on %q declares invalid mise task %q", addonID, task)
		}
		declared[task] = true
	}
	for _, value := range build.Hooks {
		hook, err := parseBuildHook(addonID, value)
		if err != nil {
			return err
		}
		if !declared[hook.Task] {
			return fmt.Errorf("add-on %q build hook %q references task %q outside its tasks list", addonID, value, hook.Task)
		}
	}
	return nil
}

func parseBuildHook(addonID string, value string) (BuildHook, error) {
	phaseValue, task, ok := strings.Cut(strings.TrimSpace(value), "=")
	phase := BuildHookPhase(strings.TrimSpace(phaseValue))
	task = strings.TrimSpace(task)
	if !ok || !validBuildHookPhase(phase) || !validMiseTaskName(task) {
		return BuildHook{}, fmt.Errorf("add-on %q has invalid build hook %q; expected before-build=<mise-task> or after-build=<mise-task>", addonID, value)
	}
	return BuildHook{AddonID: addonID, Phase: phase, Task: task}, nil
}

func validBuildHookPhase(phase BuildHookPhase) bool {
	return phase == BeforeBuild || phase == AfterBuild
}

func validMiseTaskName(task string) bool {
	if task == "" {
		return false
	}
	for _, part := range strings.Split(task, ":") {
		if part == "" || part[0] == '.' || part[0] == '_' {
			return false
		}
		for _, character := range part {
			if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("._-", character) {
				continue
			}
			return false
		}
	}
	return true
}

func allBuildHooks(state State) ([]BuildHook, error) {
	ordered, err := dependencyOrder(state.Addons)
	if err != nil {
		return nil, err
	}
	var hooks []BuildHook
	for _, addon := range ordered {
		var addonHooks []BuildHook
		for _, value := range addon.Build.Hooks {
			hook, err := parseBuildHook(addon.ID, value)
			if err != nil {
				return nil, err
			}
			hook.Target, err = declaredBuildTaskTarget(addon, hook.Task)
			if err != nil {
				return nil, err
			}
			addonHooks = append(addonHooks, hook)
		}
		sort.Slice(addonHooks, func(i, j int) bool {
			if addonHooks[i].Phase != addonHooks[j].Phase {
				return addonHooks[i].Phase < addonHooks[j].Phase
			}
			return addonHooks[i].Task < addonHooks[j].Task
		})
		hooks = append(hooks, addonHooks...)
	}
	return hooks, nil
}

// validatePlannedBuildHooks is called after portable file contributions are
// reconciled. A hook may execute only a task artifact created and owned by the
// declaring add-on; an identical pre-existing application task is not enough.
func validatePlannedBuildHooks(addons []InstalledAddon, files []ManagedFile) error {
	managed := make(map[string]ManagedFile, len(files))
	for _, file := range files {
		managed[file.Path] = file
	}
	for _, addon := range addons {
		for _, value := range addon.Build.Hooks {
			hook, err := parseBuildHook(addon.ID, value)
			if err != nil {
				return err
			}
			target, err := declaredBuildTaskTarget(addon, hook.Task)
			if err != nil {
				return err
			}
			file, ok := managed[target]
			if !ok || file.Kind != "addon-file" || !containsString(file.Owners, addon.ID) {
				return fmt.Errorf("add-on %q build hook task %q must be installed as an add-on-owned artifact at %s; pre-existing application tasks cannot be hooked", addon.ID, hook.Task, target)
			}
		}
	}
	return nil
}

func declaredBuildTaskTarget(addon InstalledAddon, task string) (string, error) {
	var targets []string
	for _, declaration := range addon.Files {
		_, target, mapped := strings.Cut(declaration, "=>")
		if !mapped {
			continue
		}
		target = strings.TrimSpace(target)
		const prefix = ".mise/tasks/"
		if !strings.HasPrefix(target, prefix) {
			continue
		}
		if taskservice.TaskNameFromFile(strings.TrimPrefix(target, prefix)) == task {
			targets = append(targets, target)
		}
	}
	targets = normalizedStrings(targets)
	if len(targets) == 0 {
		return "", fmt.Errorf("add-on %q build hook task %q is not backed by a declared source=>.mise/tasks artifact", addon.ID, task)
	}
	if len(targets) != 1 {
		return "", fmt.Errorf("add-on %q build hook task %q is ambiguous across artifacts %s", addon.ID, task, strings.Join(targets, ", "))
	}
	return targets[0], nil
}

func validateLockedBuildHooks(root string, state State, hooks []BuildHook) error {
	for _, hook := range hooks {
		if err := validateAddonTarget(root, hook.Target); err != nil {
			return err
		}
		managed, ok := state.managedFile(hook.Target)
		if !ok || managed.Kind != "addon-file" || !containsString(managed.Owners, hook.AddonID) {
			return fmt.Errorf("add-on %q build hook task %q is not locked as an add-on-owned artifact at %s", hook.AddonID, hook.Task, hook.Target)
		}
		filename := filepath.Join(root, filepath.FromSlash(hook.Target))
		info, err := os.Lstat(filename)
		if err != nil {
			return fmt.Errorf("inspect add-on %q build hook task %q: %w", hook.AddonID, hook.Task, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("add-on %q build hook task %q at %s is not a regular file", hook.AddonID, hook.Task, hook.Target)
		}
		if managed.Mode == 0 || info.Mode().Perm() != os.FileMode(managed.Mode).Perm() || info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("add-on %q build hook task %q at %s has mode %o; expected executable mode %o", hook.AddonID, hook.Task, hook.Target, info.Mode().Perm(), os.FileMode(managed.Mode).Perm())
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read add-on %q build hook task %q: %w", hook.AddonID, hook.Task, err)
		}
		if lazycode.Hash(data) != managed.Baseline {
			return fmt.Errorf("add-on %q build hook task %q has local changes; expected %s", hook.AddonID, hook.Task, managed.Baseline)
		}
	}
	return nil
}

// dependencyOrder implements a stable topological order. Among add-ons whose
// requirements are already satisfied, the lexicographically smallest ID runs
// first.
func dependencyOrder(addons []InstalledAddon) ([]InstalledAddon, error) {
	byID := make(map[string]InstalledAddon, len(addons))
	indegree := make(map[string]int, len(addons))
	dependents := make(map[string][]string, len(addons))
	for _, addon := range addons {
		if _, exists := byID[addon.ID]; exists {
			return nil, fmt.Errorf("add-on %q is duplicated", addon.ID)
		}
		byID[addon.ID] = addon
		indegree[addon.ID] = 0
	}
	for _, addon := range addons {
		for _, requirement := range addon.Requires {
			required, version, err := ParseSpec(requirement)
			if err != nil || version == "" {
				return nil, fmt.Errorf("add-on %q declares invalid or unpinned requirement %q", addon.ID, requirement)
			}
			dependency, exists := byID[required]
			if !exists {
				return nil, fmt.Errorf("add-on %q requires missing add-on %q", addon.ID, required)
			}
			if dependency.Version != version {
				return nil, fmt.Errorf("add-on %q requires add-on %q at %s, but the lock contains %s", addon.ID, required, version, dependency.Version)
			}
			indegree[addon.ID]++
			dependents[required] = append(dependents[required], addon.ID)
		}
	}
	ready := make([]string, 0, len(addons))
	for id, count := range indegree {
		if count == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	ordered := make([]InstalledAddon, 0, len(addons))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byID[id])
		sort.Strings(dependents[id])
		for _, dependent := range dependents[id] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) != len(addons) {
		return nil, fmt.Errorf("add-on dependency cycle prevents build hook ordering")
	}
	return ordered, nil
}
