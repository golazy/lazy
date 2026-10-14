package upgradeservice

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
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

func (e stepExecutor) repairLegacyGenprotoParent() error {
	data, err := os.ReadFile(filepath.Join(e.dir, "go.mod"))
	if err != nil {
		return err
	}
	file, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return err
	}
	current := directRequirements(file)
	// Older app tools can retain the pre-split parent beside the API/RPC
	// submodules introduced by telemetry. Google's supported split baseline
	// removes the duplicate packages without downgrading newer app versions.
	const parent = "google.golang.org/genproto"
	const splitBaseline = "v0.0.0-20230526161137-0005af68ea54"
	version := current[parent]
	if version == "" && semver.Compare(e.to, "v0.1.20") >= 0 && !e.dryRun {
		var selected bytes.Buffer
		if err := e.runner("go", []string{"list", "-m", "-e", "-f", "{{.Version}}", parent}, execservice.Options{Dir: e.dir, Stdout: &selected, Stderr: e.stderr}); err != nil {
			return fmt.Errorf("inspect genproto dependency: %w", err)
		}
		version = strings.TrimSpace(selected.String())
	}
	if semver.Compare(e.to, "v0.1.20") >= 0 && semver.IsValid(version) && semver.Compare(version, splitBaseline) < 0 {
		for _, replacement := range file.Replace {
			if replacement.Old.Path == parent && (replacement.Old.Version == "" || replacement.Old.Version == version) {
				return nil // Keep an application-owned override intact.
			}
		}
		if err := e.runGoGet(parent + "@" + splitBaseline); err != nil {
			return err
		}
		// Workspace sync can prune the indirect parent requirement again. Keep
		// only the incompatible old version mapped to the split baseline, so
		// newer selected versions are never overridden.
		replacement := parent + "@" + version + "=" + parent + "@" + splitBaseline
		fmt.Fprintf(e.stdout, "  pinning pre-split parent %s\n", replacement)
		return e.runner("go", []string{"mod", "edit", "-replace=" + replacement}, execservice.Options{Dir: e.dir, Stdout: e.stdout, Stderr: e.stderr})
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
