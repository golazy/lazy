package addonservice

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type lockedSnapshotSource interface {
	allowMissingLockedArtifacts()
}

type fileArtifactKey struct {
	source string
	target string
}

type artifactDeclarations struct {
	files map[fileArtifactKey]struct{}
	edits map[string]struct{}
}

// validateSourceArtifacts binds transient bytes supplied by a Source to the
// exact portable declarations that authorize them. StateSource is the only
// exception: addons.toml deliberately stores declarations and output hashes,
// not package bytes, so an offline exact-pin resolution may omit all transient
// artifacts and let the planner preserve the already verified application
// outputs.
func validateSourceArtifacts(addon ResolvedAddon, allowMissing bool) error {
	declared, err := validateArtifactDeclarations(addon)
	if err != nil {
		return err
	}
	if err := validateFileArtifacts(addon, declared.files, allowMissing); err != nil {
		return err
	}
	return validateSourceEditArtifacts(addon, declared.edits, allowMissing)
}

func validateArtifactDeclarations(addon ResolvedAddon) (artifactDeclarations, error) {
	declarations := artifactDeclarations{
		files: make(map[fileArtifactKey]struct{}),
		edits: make(map[string]struct{}),
	}
	addonID := strings.TrimSpace(addon.ID)
	seenFiles := make(map[string]struct{}, len(addon.Files))
	seenTargets := make(map[string]string, len(addon.Files))
	for _, raw := range addon.Files {
		declaration := strings.TrimSpace(raw)
		if declaration == "" {
			return declarations, fmt.Errorf("add-on %q declares an empty file", addonID)
		}
		if _, exists := seenFiles[declaration]; exists {
			return declarations, fmt.Errorf("add-on %q declares file %q more than once", addonID, declaration)
		}
		seenFiles[declaration] = struct{}{}

		source, target, mapped := strings.Cut(declaration, "=>")
		if mapped {
			var err error
			source, target, err = parseFileMapping(declaration)
			if err != nil {
				return declarations, fmt.Errorf("add-on %q declares invalid file mapping %q: %w", addonID, declaration, err)
			}
			key := fileArtifactKey{source: source, target: target}
			if _, exists := declarations.files[key]; exists {
				return declarations, fmt.Errorf("add-on %q declares file mapping %q more than once", addonID, declaration)
			}
			declarations.files[key] = struct{}{}
		} else {
			target = declaration
			if err := validateDeclaredFileTarget(target); err != nil {
				return declarations, fmt.Errorf("add-on %q declares invalid file target %q: %w", addonID, target, err)
			}
		}
		if previous, exists := seenTargets[target]; exists {
			return declarations, fmt.Errorf("add-on %q declares target %q more than once (%q and %q)", addonID, target, previous, declaration)
		}
		seenTargets[target] = declaration
	}

	for _, raw := range addon.SourceEdits {
		reference := strings.TrimSpace(raw)
		if err := validateSourceEditReference(reference); err != nil {
			return declarations, fmt.Errorf("add-on %q declares invalid source edit %q: %w", addonID, reference, err)
		}
		if _, exists := declarations.edits[reference]; exists {
			return declarations, fmt.Errorf("add-on %q declares source edit %q more than once", addonID, reference)
		}
		declarations.edits[reference] = struct{}{}
	}
	return declarations, nil
}

func validateDeclaredFileTarget(target string) error {
	if !validFilePath(target) {
		return fmt.Errorf("invalid relative target path")
	}
	for reserved := range reservedContributionTargets {
		if strings.EqualFold(target, reserved) {
			return fmt.Errorf("target is installer-owned; use its structured contribution")
		}
	}
	if path.Ext(target) == ".go" {
		base := path.Base(target)
		if strings.HasPrefix(base, "_") || strings.HasPrefix(base, ".") {
			return fmt.Errorf("Go target would be ignored by the Go toolchain")
		}
	}
	first, _, _ := strings.Cut(target, "/")
	if strings.EqualFold(first, ".git") || strings.EqualFold(first, ".secrets") {
		return fmt.Errorf("target is outside the application source surface")
	}
	return nil
}

func validateFileArtifacts(addon ResolvedAddon, declared map[fileArtifactKey]struct{}, allowMissing bool) error {
	addonID := strings.TrimSpace(addon.ID)
	seen := make(map[fileArtifactKey]struct{}, len(addon.Artifacts))
	for _, artifact := range addon.Artifacts {
		source, target, err := parseFileMapping(artifact.Source + "=>" + artifact.Target)
		if err != nil {
			return fmt.Errorf("add-on %q file artifact %q=>%q has invalid metadata: %w", addonID, artifact.Source, artifact.Target, err)
		}
		if source != artifact.Source || target != artifact.Target {
			return fmt.Errorf("add-on %q file artifact %q=>%q has non-canonical metadata", addonID, artifact.Source, artifact.Target)
		}
		if artifact.Mode != 0o644 && artifact.Mode != 0o755 {
			return fmt.Errorf("add-on %q file artifact %q=>%q has invalid mode %o; expected 644 or 755", addonID, source, target, artifact.Mode)
		}
		key := fileArtifactKey{source: source, target: target}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("add-on %q provides file artifact %q=>%q more than once", addonID, source, target)
		}
		if _, exists := declared[key]; !exists {
			return fmt.Errorf("add-on %q provides undeclared or mismatched file artifact %q=>%q", addonID, source, target)
		}
		seen[key] = struct{}{}
	}
	if allowMissing && len(addon.Artifacts) == 0 {
		return nil
	}
	for _, key := range sortedFileArtifactKeys(declared) {
		if _, exists := seen[key]; !exists {
			return fmt.Errorf("add-on %q did not provide declared file artifact %q=>%q", addonID, key.source, key.target)
		}
	}
	return nil
}

func sortedFileArtifactKeys(values map[fileArtifactKey]struct{}) []fileArtifactKey {
	keys := make([]fileArtifactKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].target != keys[j].target {
			return keys[i].target < keys[j].target
		}
		return keys[i].source < keys[j].source
	})
	return keys
}

func validateSourceEditArtifacts(addon ResolvedAddon, declared map[string]struct{}, allowMissing bool) error {
	addonID := strings.TrimSpace(addon.ID)
	seen := make(map[string]struct{}, len(addon.EditArtifacts))
	for _, artifact := range addon.EditArtifacts {
		reference := strings.TrimSpace(artifact.Source)
		if reference != artifact.Source {
			return fmt.Errorf("add-on %q source edit artifact %q has non-canonical metadata", addonID, artifact.Source)
		}
		if err := validateSourceEditReference(reference); err != nil {
			return fmt.Errorf("add-on %q source edit artifact %q is invalid: %w", addonID, reference, err)
		}
		if _, exists := seen[reference]; exists {
			return fmt.Errorf("add-on %q provides source edit artifact %q more than once", addonID, reference)
		}
		if _, exists := declared[reference]; !exists {
			return fmt.Errorf("add-on %q provides undeclared or mismatched source edit artifact %q", addonID, reference)
		}
		if _, err := parseSourceEditDescriptor(artifact.Data, reference); err != nil {
			return fmt.Errorf("add-on %q source edit %q: %w", addonID, reference, err)
		}
		seen[reference] = struct{}{}
	}
	if allowMissing && len(addon.EditArtifacts) == 0 {
		return nil
	}
	references := make([]string, 0, len(declared))
	for reference := range declared {
		references = append(references, reference)
	}
	sort.Strings(references)
	for _, reference := range references {
		if _, exists := seen[reference]; !exists {
			return fmt.Errorf("add-on %q did not provide declared source edit artifact %q", addonID, reference)
		}
	}
	return nil
}
