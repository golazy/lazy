// Package addonservice plans and applies manifest-driven GoLazy add-on changes.
package addonservice

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const (
	StateFilename       = "addons.toml"
	GeneratedFilename   = "init/addons_gen.go"
	ApplicationFilename = "init/app.go"
	GoModFilename       = "go.mod"
	// InstallerFrameworkVersion is the minimum runtime contract emitted by
	// this CLI add-on installer.
	InstallerFrameworkVersion = "v0.1.21"
)

// Query identifies one add-on version. An empty Version asks a source for its
// newest exact version.
type Query struct {
	ID      string
	Version string
}

// Source is the distribution boundary used by the installer. Local manifest
// files implement it today; a signed registry or marketplace can implement the
// same query contract without changing planning or application code.
type Source interface {
	ID() string
	Resolve(context.Context, Query) ([]ResolvedAddon, error)
}

// FileArtifact is one verified local manifest file contribution. Data and Mode
// are transient installer inputs: addons.toml keeps the mapping and package
// digest, while a later update reloads the bytes from the recorded source.
type FileArtifact struct {
	Source string
	Target string
	Data   []byte
	Mode   uint32
}

// SourceEditArtifact is a verified, package-local declarative edit descriptor.
// Data is transient installer input; the locked source_edits paths and package
// digest make the selected release reproducible without embedding opaque code.
type SourceEditArtifact struct {
	Source string
	Data   []byte
}

// ResolvedAddon is the install-relevant part of one lazyaddon.toml entry.
type ResolvedAddon struct {
	ID              string
	Version         string
	Description     string
	Requires        []string
	Optional        []string
	Conflicts       []string
	Package         string
	PackageVersion  string
	ManifestDigest  string
	Source          string
	GoImport        string
	GoModule        string
	GoModuleVersion string
	Hooks           []string
	JavaScript      JavaScriptContributions
	Build           BuildContributions
	Files           []string
	Artifacts       []FileArtifact
	SourceEdits     []string
	EditArtifacts   []SourceEditArtifact
}

// JavaScriptContributions records structured, reversible js.toml and
// package.json contributions. Libraries use package@exact-version and
// entrypoints use name=module.
type JavaScriptContributions struct {
	Libraries   []string
	Entrypoints []string
}

// BuildContributions records build hook and mise task declarations.
type BuildContributions struct {
	Hooks []string
	Tasks []string
}

// State is both the desired add-on configuration and its exact lock data.
// Every installed package/module version is concrete; there is no separate
// lock file.
type State struct {
	Manifest int
	Addons   []InstalledAddon
	Modules  []ManagedModule
	Managed  []ManagedFile
}

// InstalledAddon is one selected or required add-on pinned in addons.toml.
type InstalledAddon struct {
	ResolvedAddon
	Direct bool
	// Config is non-secret, application-specific runtime configuration. It is
	// desired state rather than package metadata and is therefore stored only
	// in the application's addons.toml lock.
	Config map[string]string
}

// ManagedModule tracks semantic ownership of a go.mod requirement. Previous
// is restored when the last owner is removed, provided the locked version has
// not drifted.
type ManagedModule struct {
	Path             string
	Version          string
	Owners           []string
	Managed          bool
	Previous         string
	PreviousIndirect bool
}

// ManagedFile records the expected output hash for generated and rewritten
// files. Generated files are drift-protected before subsequent plans.
type ManagedFile struct {
	Path       string
	Kind       string
	Owners     []string
	Baseline   string
	Mode       uint32
	Properties []string
}

func (state State) addon(id string) (InstalledAddon, bool) {
	for _, addon := range state.Addons {
		if addon.ID == id {
			return addon, true
		}
	}
	return InstalledAddon{}, false
}

func (state State) module(path string) (ManagedModule, bool) {
	for _, module := range state.Modules {
		if module.Path == path {
			return module, true
		}
	}
	return ManagedModule{}, false
}

func (state State) managedFile(path string) (ManagedFile, bool) {
	for _, file := range state.Managed {
		if file.Path == path {
			return file, true
		}
	}
	return ManagedFile{}, false
}

func normalizeState(state *State) {
	if state.Manifest == 0 {
		state.Manifest = 1
	}
	sort.Slice(state.Addons, func(i, j int) bool { return state.Addons[i].ID < state.Addons[j].ID })
	for index := range state.Addons {
		normalizeResolved(&state.Addons[index].ResolvedAddon)
		state.Addons[index].Config = cloneConfig(state.Addons[index].Config)
	}
	sort.Slice(state.Modules, func(i, j int) bool { return state.Modules[i].Path < state.Modules[j].Path })
	for index := range state.Modules {
		state.Modules[index].Owners = normalizedStrings(state.Modules[index].Owners)
	}
	sort.Slice(state.Managed, func(i, j int) bool { return state.Managed[i].Path < state.Managed[j].Path })
	for index := range state.Managed {
		state.Managed[index].Owners = normalizedStrings(state.Managed[index].Owners)
		state.Managed[index].Properties = normalizedStrings(state.Managed[index].Properties)
	}
}

func cloneConfig(config map[string]string) map[string]string {
	if len(config) == 0 {
		return nil
	}
	clone := make(map[string]string, len(config))
	for key, value := range config {
		clone[key] = value
	}
	return clone
}

func validateAddonConfig(config map[string]string) error {
	for key := range config {
		if !validConfigKey(key) {
			return fmt.Errorf("add-on configuration key %q is invalid", key)
		}
	}
	return nil
}

func validConfigKey(key string) bool {
	if key == "" {
		return false
	}
	for index, character := range key {
		letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		digit := character >= '0' && character <= '9'
		if letter || index > 0 && (digit || strings.ContainsRune("._-", character)) {
			continue
		}
		return false
	}
	return true
}

func normalizeResolved(addon *ResolvedAddon) {
	addon.ID = strings.TrimSpace(addon.ID)
	addon.Version = strings.TrimSpace(addon.Version)
	addon.Package = strings.TrimSpace(addon.Package)
	addon.PackageVersion = strings.TrimSpace(addon.PackageVersion)
	addon.ManifestDigest = strings.TrimSpace(addon.ManifestDigest)
	addon.Source = strings.TrimSpace(addon.Source)
	addon.GoImport = strings.TrimSpace(addon.GoImport)
	addon.GoModule = strings.TrimSpace(addon.GoModule)
	addon.GoModuleVersion = strings.TrimSpace(addon.GoModuleVersion)
	addon.Requires = normalizedRequirements(addon.Requires, addon.PackageVersion)
	addon.Optional = normalizedStrings(addon.Optional)
	addon.Conflicts = normalizedStrings(addon.Conflicts)
	addon.Hooks = normalizedStrings(addon.Hooks)
	addon.JavaScript.Libraries = normalizedStrings(addon.JavaScript.Libraries)
	addon.JavaScript.Entrypoints = normalizedStrings(addon.JavaScript.Entrypoints)
	addon.Build.Hooks = normalizedStrings(addon.Build.Hooks)
	addon.Build.Tasks = normalizedStrings(addon.Build.Tasks)
	addon.Files = normalizedStrings(addon.Files)
	addon.SourceEdits = normalizedStrings(addon.SourceEdits)
	sort.Slice(addon.Artifacts, func(i, j int) bool {
		if addon.Artifacts[i].Target != addon.Artifacts[j].Target {
			return addon.Artifacts[i].Target < addon.Artifacts[j].Target
		}
		return addon.Artifacts[i].Source < addon.Artifacts[j].Source
	})
	sort.Slice(addon.EditArtifacts, func(i, j int) bool {
		return addon.EditArtifacts[i].Source < addon.EditArtifacts[j].Source
	})
}

func normalizedRequirements(requirements []string, defaultVersion string) []string {
	values := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		id, version, err := ParseSpec(requirement)
		if err != nil {
			values = append(values, requirement)
			continue
		}
		if version == "" {
			version = strings.TrimSpace(defaultVersion)
		}
		if version != "" {
			values = append(values, id+"@"+version)
		} else {
			values = append(values, id)
		}
	}
	return normalizedStrings(values)
}

func validateResolved(addon ResolvedAddon) error {
	if addon.ID == "" {
		return fmt.Errorf("add-on ID is required")
	}
	if !validAddonID(addon.ID) {
		return fmt.Errorf("add-on ID %q is invalid", addon.ID)
	}
	if !exactVersion(addon.Version) {
		return fmt.Errorf("add-on %q has non-exact version %q", addon.ID, addon.Version)
	}
	if addon.Package == "" || !exactVersion(addon.PackageVersion) {
		return fmt.Errorf("add-on %q must pin an exact package and package version", addon.ID)
	}
	if !validSHA256Digest(addon.ManifestDigest) {
		return fmt.Errorf("add-on %q must provide a SHA-256 manifest digest", addon.ID)
	}
	if addon.Source == "" {
		return fmt.Errorf("add-on %q source is required", addon.ID)
	}
	if err := module.CheckPath(addon.Package); err != nil {
		return fmt.Errorf("add-on %q package path: %w", addon.ID, err)
	}
	if addon.GoImport == "" {
		return fmt.Errorf("add-on %q does not declare go_import", addon.ID)
	}
	if err := module.CheckImportPath(addon.GoImport); err != nil {
		return fmt.Errorf("add-on %q go_import: %w", addon.ID, err)
	}
	if addon.GoModule == "" || !exactVersion(addon.GoModuleVersion) {
		return fmt.Errorf("add-on %q must pin go_module and an exact go_version", addon.ID)
	}
	if err := module.Check(addon.GoModule, addon.GoModuleVersion); err != nil {
		return fmt.Errorf("add-on %q Go module: %w", addon.ID, err)
	}
	if addon.GoImport != addon.GoModule && !strings.HasPrefix(addon.GoImport, addon.GoModule+"/") {
		return fmt.Errorf("add-on %q go_import %q is outside go_module %q", addon.ID, addon.GoImport, addon.GoModule)
	}
	for _, requirement := range addon.Requires {
		required, version, err := ParseSpec(requirement)
		if err != nil || !validAddonID(required) || version == "" {
			return fmt.Errorf("add-on %q declares invalid or unpinned requirement %q", addon.ID, requirement)
		}
	}
	for _, related := range append(append([]string(nil), addon.Optional...), addon.Conflicts...) {
		if !validAddonID(related) {
			return fmt.Errorf("add-on %q references invalid add-on ID %q", addon.ID, related)
		}
	}
	for _, file := range addon.Files {
		if _, _, mapped := strings.Cut(file, "=>"); mapped {
			source, target, err := parseFileMapping(file)
			if err != nil {
				return fmt.Errorf("add-on %q declares invalid file mapping %q: %w", addon.ID, file, err)
			}
			if source == "" || target == "" {
				return fmt.Errorf("add-on %q declares invalid file mapping %q", addon.ID, file)
			}
			continue
		}
		if !validFilePath(file) {
			return fmt.Errorf("add-on %q declares invalid file path %q", addon.ID, file)
		}
	}
	for _, descriptor := range addon.SourceEdits {
		if err := validateSourceEditReference(descriptor); err != nil {
			return fmt.Errorf("add-on %q declares invalid source edit %q: %w", addon.ID, descriptor, err)
		}
	}
	if len(addon.EditArtifacts) != 0 {
		artifacts := map[string]bool{}
		for _, artifact := range addon.EditArtifacts {
			if artifacts[artifact.Source] || !containsString(addon.SourceEdits, artifact.Source) {
				return fmt.Errorf("add-on %q has unexpected or duplicate source edit artifact %q", addon.ID, artifact.Source)
			}
			artifacts[artifact.Source] = true
			if _, err := parseSourceEditDescriptor(artifact.Data, artifact.Source); err != nil {
				return fmt.Errorf("add-on %q source edit %q: %w", addon.ID, artifact.Source, err)
			}
		}
		if len(artifacts) != len(addon.SourceEdits) {
			return fmt.Errorf("add-on %q did not provide every declared source edit artifact", addon.ID)
		}
	}
	if err := validateJavaScriptContributions(addon.ID, addon.JavaScript); err != nil {
		return err
	}
	if err := validateBuildContributions(addon.ID, addon.Build); err != nil {
		return err
	}
	return nil
}

func validFilePath(value string) bool {
	return value != "." && !strings.ContainsAny(value, "\\:") && fs.ValidPath(value)
}

func validModuleVersion(path, version string) bool {
	return module.Check(path, version) == nil
}

func validAddonID(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "//") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, character := range part {
			if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || strings.ContainsRune("._-", character) {
				continue
			}
			return false
		}
	}
	return true
}

func exactVersion(value string) bool {
	value = strings.TrimSpace(value)
	return semver.IsValid(value)
}

func validSHA256Digest(value string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+64 {
		return false
	}
	digest := strings.TrimPrefix(value, prefix)
	if digest != strings.ToLower(digest) {
		return false
	}
	decoded, err := hex.DecodeString(digest)
	return err == nil && len(decoded) == 32
}

func normalizedStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
