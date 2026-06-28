package upgradeservice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golazy.dev/lazy/services/execservice"
)

type upgradeGoModManifest struct {
	From         string
	To           string
	Requirements []upgradeGoModRequirement
}

type upgradeGoModRequirement struct {
	Path     string
	Previous string
	Target   string
}

func goModManifestFor(from string, to string) upgradeGoModManifest {
	return upgradeGoModManifest{
		From: from,
		To:   to,
		Requirements: []upgradeGoModRequirement{
			{Path: "golazy.dev", Previous: from, Target: to},
		},
	}
}

func (e stepExecutor) applyGoModManifest(manifest upgradeGoModManifest) error {
	if manifest.From != "" && manifest.From != e.from {
		return fmt.Errorf("upgrade go.mod manifest starts at %s, want %s", manifest.From, e.from)
	}
	if manifest.To != "" && manifest.To != e.to {
		return fmt.Errorf("upgrade go.mod manifest targets %s, want %s", manifest.To, e.to)
	}
	path := filepath.Join(e.dir, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read go.mod: %w", err)
	}
	file, err := modfile.Parse(path, data, nil)
	if err != nil {
		return fmt.Errorf("parse go.mod: %w", err)
	}
	current := directRequirements(file)
	for _, requirement := range manifest.Requirements {
		if err := e.applyGoModRequirement(requirement, current[requirement.Path]); err != nil {
			return err
		}
	}
	return nil
}

func directRequirements(file *modfile.File) map[string]string {
	result := make(map[string]string)
	for _, require := range file.Require {
		result[require.Mod.Path] = require.Mod.Version
	}
	return result
}

func (e stepExecutor) applyGoModRequirement(requirement upgradeGoModRequirement, current string) error {
	if strings.TrimSpace(requirement.Path) == "" {
		return fmt.Errorf("go.mod manifest requirement path is required")
	}
	if requirement.Target == "" {
		return e.dropGoModRequirement(requirement, current)
	}
	return e.setGoModRequirement(requirement, current)
}

func (e stepExecutor) setGoModRequirement(requirement upgradeGoModRequirement, current string) error {
	switch current {
	case requirement.Target:
		fmt.Fprintf(e.stdout, "  go.mod already requires %s %s\n", requirement.Path, requirement.Target)
		return nil
	case "", requirement.Previous:
		// Continue below.
	default:
		if requirement.Previous != "" {
			fmt.Fprintf(e.stdout, "  go.mod requires %s %s; manifest expected %s before %s\n", requirement.Path, current, requirement.Previous, e.to)
		}
	}
	return e.runGoGet(requirement.Path + "@" + requirement.Target)
}

func (e stepExecutor) dropGoModRequirement(requirement upgradeGoModRequirement, current string) error {
	if current == "" {
		fmt.Fprintf(e.stdout, "  go.mod already omits %s\n", requirement.Path)
		return nil
	}
	if requirement.Previous != "" && current != requirement.Previous {
		return fmt.Errorf("go.mod requires %s %s, want %s before removing it for %s", requirement.Path, current, requirement.Previous, e.to)
	}
	return e.runGoGet(requirement.Path + "@none")
}

func (e stepExecutor) runGoGet(spec string) error {
	if e.dryRun {
		fmt.Fprintf(e.stdout, "  would run go get %s\n", spec)
		return nil
	}
	fmt.Fprintf(e.stdout, "  running go get %s\n", spec)
	if err := e.runner("go", []string{"get", spec}, execservice.Options{
		Dir:    e.dir,
		Stdout: e.stdout,
		Stderr: e.stderr,
	}); err != nil {
		return fmt.Errorf("go get %s: %w", spec, err)
	}
	return nil
}
