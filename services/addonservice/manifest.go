package addonservice

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golazy.dev/lazyaddon"
)

// PackageManifest is the install-time view of lazyaddon.toml.
type PackageManifest struct {
	Package string
	Version string
	Digest  string
	Addons  []ResolvedAddon
}

// ParsePackageManifest reads core dependency metadata with lazyaddon and the
// install contribution keys understood by the CLI. Contribution keys live on
// each [[addons]] entry so runtime and installer metadata stay together.
func ParsePackageManifest(data []byte, source string) (PackageManifest, error) {
	runtimeManifest, err := lazyaddon.ParseManifest(data)
	if err != nil {
		return PackageManifest{}, err
	}
	manifest := PackageManifest{
		Package: runtimeManifest.Package.ID,
		Version: runtimeManifest.Package.Version,
		Digest:  runtimeManifest.Digest,
		Addons:  make([]ResolvedAddon, len(runtimeManifest.Addons)),
	}
	for index, definition := range runtimeManifest.Addons {
		version := definition.Version
		if version == "" {
			version = runtimeManifest.Package.Version
		}
		manifest.Addons[index] = ResolvedAddon{
			ID:             definition.ID,
			Version:        version,
			Description:    definition.Description,
			Requires:       definition.Requires,
			Optional:       definition.Optional,
			Conflicts:      definition.Conflicts,
			Package:        runtimeManifest.Package.ID,
			PackageVersion: runtimeManifest.Package.Version,
			ManifestDigest: runtimeManifest.Digest,
			Source:         source,
		}
	}

	addonIndex := -1
	for lineIndex, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(stripComment(rawLine))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[[") {
			if line == "[[addons]]" {
				addonIndex++
			} else {
				addonIndex = -1
			}
			continue
		}
		if strings.HasPrefix(line, "[") {
			addonIndex = -1
			continue
		}
		if addonIndex < 0 || addonIndex >= len(manifest.Addons) {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		addon := &manifest.Addons[addonIndex]
		var parseErr error
		switch key {
		case "go_import":
			addon.GoImport, parseErr = parseString(value)
		case "go_module":
			addon.GoModule, parseErr = parseString(value)
		case "go_version", "go_module_version":
			addon.GoModuleVersion, parseErr = parseString(value)
		case "hooks":
			addon.Hooks, parseErr = parseStrings(value)
		case "js_libraries":
			addon.JavaScript.Libraries, parseErr = parseStrings(value)
		case "js_entrypoints":
			addon.JavaScript.Entrypoints, parseErr = parseStrings(value)
		case "build_hooks":
			addon.Build.Hooks, parseErr = parseStrings(value)
		case "tasks":
			addon.Build.Tasks, parseErr = parseStrings(value)
		case "files":
			addon.Files, parseErr = parseStrings(value)
		case "source_edits":
			addon.SourceEdits, parseErr = parseStrings(value)
		}
		if parseErr != nil {
			return PackageManifest{}, fmt.Errorf("lazyaddon.toml line %d: %s: %w", lineIndex+1, key, parseErr)
		}
	}

	for index := range manifest.Addons {
		addon := &manifest.Addons[index]
		if addon.GoModule == "" {
			addon.GoModule = manifest.Package
		}
		if addon.GoModuleVersion == "" {
			addon.GoModuleVersion = manifest.Version
		}
		if _, err := validateArtifactDeclarations(*addon); err != nil {
			return PackageManifest{}, fmt.Errorf("manifest add-on %q: %w", addon.ID, err)
		}
		normalizeResolved(addon)
		if err := validateResolved(*addon); err != nil {
			return PackageManifest{}, fmt.Errorf("manifest add-on %q: %w", addon.ID, err)
		}
	}
	return manifest, nil
}

// LocalManifestSource resolves add-ons from one lazyaddon.toml file.
type LocalManifestSource struct {
	Root string
	Path string
}

func (source LocalManifestSource) ID() string {
	return "local:" + source.relativePath()
}

func (source LocalManifestSource) Resolve(ctx context.Context, query Query) ([]ResolvedAddon, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := source.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(source.Root, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read add-on manifest %s: %w", source.relativePath(), err)
	}
	manifest, err := ParsePackageManifest(data, source.ID())
	if err != nil {
		return nil, fmt.Errorf("parse add-on manifest %s: %w", source.relativePath(), err)
	}
	if err := loadLocalArtifacts(ctx, path, data, &manifest); err != nil {
		return nil, fmt.Errorf("load add-on artifacts %s: %w", source.relativePath(), err)
	}
	var matches []ResolvedAddon
	for _, addon := range manifest.Addons {
		if addon.ID != query.ID || query.Version != "" && addon.Version != query.Version {
			continue
		}
		matches = append(matches, addon)
	}
	return matches, nil
}

func (source LocalManifestSource) relativePath() string {
	path := filepath.Clean(source.Path)
	if filepath.IsAbs(path) && source.Root != "" {
		if relative, err := filepath.Rel(source.Root, path); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			path = relative
		}
	}
	return filepath.ToSlash(path)
}

// MemorySource is useful for compiled first-party catalogs and tests. A
// marketplace client can expose the same Source interface while retaining its
// own transport and trust policy.
type MemorySource struct {
	Name   string
	Addons []ResolvedAddon
}

func (source MemorySource) ID() string { return source.Name }

func (source MemorySource) Resolve(ctx context.Context, query Query) ([]ResolvedAddon, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var matches []ResolvedAddon
	for _, addon := range source.Addons {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if addon.ID != query.ID || query.Version != "" && addon.Version != query.Version {
			continue
		}
		addon.Source = source.ID()
		if err := validateSourceArtifacts(addon, false); err != nil {
			return nil, err
		}
		normalizeResolved(&addon)
		if err := validateResolved(addon); err != nil {
			return nil, err
		}
		matches = append(matches, addon)
	}
	return matches, nil
}

func loadLocalArtifacts(ctx context.Context, manifestPath string, manifestData []byte, manifest *PackageManifest) error {
	manifestDir, err := filepath.Abs(filepath.Dir(manifestPath))
	if err != nil {
		return err
	}
	realManifestDir, err := filepath.EvalSymlinks(manifestDir)
	if err != nil {
		return fmt.Errorf("resolve manifest directory: %w", err)
	}
	digest := sha256.New()
	writeDigestPart(digest, "manifest", manifestData)
	for addonIndex := range manifest.Addons {
		addon := &manifest.Addons[addonIndex]
		seenTargets := map[string]bool{}
		for _, declaration := range addon.Files {
			sourcePath, targetPath, mapped := strings.Cut(declaration, "=>")
			if !mapped {
				// A plain path is locked metadata for a typed first-party planner.
				// Portable manifest file contributions always use source=>target.
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			sourcePath, targetPath, err = parseFileMapping(declaration)
			if err != nil {
				return fmt.Errorf("add-on %q file %q: %w", addon.ID, declaration, err)
			}
			if seenTargets[targetPath] {
				return fmt.Errorf("add-on %q contributes target %q more than once", addon.ID, targetPath)
			}
			seenTargets[targetPath] = true

			candidate := filepath.Join(realManifestDir, filepath.FromSlash(sourcePath))
			realCandidate, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				return fmt.Errorf("resolve source %q: %w", sourcePath, err)
			}
			if !pathWithin(realManifestDir, realCandidate) {
				return fmt.Errorf("source %q resolves outside the manifest directory", sourcePath)
			}
			info, err := os.Stat(realCandidate)
			if err != nil {
				return fmt.Errorf("inspect source %q: %w", sourcePath, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("source %q is not a regular file", sourcePath)
			}
			data, err := os.ReadFile(realCandidate)
			if err != nil {
				return fmt.Errorf("read source %q: %w", sourcePath, err)
			}
			data, err = prepareContributedFile(targetPath, data)
			if err != nil {
				return fmt.Errorf("prepare %q for %q: %w", sourcePath, targetPath, err)
			}
			mode := uint32(0o644)
			if info.Mode().Perm()&0o111 != 0 {
				mode = 0o755
			}
			artifact := FileArtifact{Source: sourcePath, Target: targetPath, Data: data, Mode: mode}
			addon.Artifacts = append(addon.Artifacts, artifact)
			writeDigestPart(digest, addon.ID+"\x00"+sourcePath+"\x00"+targetPath+fmt.Sprintf("\x00%o", mode), data)
		}
		for _, sourcePath := range addon.SourceEdits {
			if err := ctx.Err(); err != nil {
				return err
			}
			data, err := loadConfinedManifestFile(realManifestDir, sourcePath)
			if err != nil {
				return fmt.Errorf("add-on %q source edit %q: %w", addon.ID, sourcePath, err)
			}
			if _, err := parseSourceEditDescriptor(data, sourcePath); err != nil {
				return fmt.Errorf("add-on %q source edit %q: %w", addon.ID, sourcePath, err)
			}
			addon.EditArtifacts = append(addon.EditArtifacts, SourceEditArtifact{Source: sourcePath, Data: data})
			writeDigestPart(digest, addon.ID+"\x00source-edit\x00"+sourcePath, data)
		}
	}
	manifest.Digest = fmt.Sprintf("sha256:%x", digest.Sum(nil))
	for index := range manifest.Addons {
		manifest.Addons[index].ManifestDigest = manifest.Digest
		if err := validateSourceArtifacts(manifest.Addons[index], false); err != nil {
			return fmt.Errorf("add-on %q artifacts: %w", manifest.Addons[index].ID, err)
		}
		normalizeResolved(&manifest.Addons[index])
	}
	return nil
}

func loadConfinedManifestFile(realManifestDir, sourcePath string) ([]byte, error) {
	candidate := filepath.Join(realManifestDir, filepath.FromSlash(sourcePath))
	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return nil, fmt.Errorf("resolve source: %w", err)
	}
	if !pathWithin(realManifestDir, realCandidate) {
		return nil, fmt.Errorf("source resolves outside the manifest directory")
	}
	info, err := os.Stat(realCandidate)
	if err != nil {
		return nil, fmt.Errorf("inspect source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("source is not a regular file")
	}
	data, err := os.ReadFile(realCandidate)
	if err != nil {
		return nil, fmt.Errorf("read source: %w", err)
	}
	return data, nil
}

func writeDigestPart(digest hash.Hash, label string, data []byte) {
	fmt.Fprintf(digest, "%d:%s:%d:", len(label), label, len(data))
	_, _ = digest.Write(data)
	_, _ = digest.Write([]byte{'\n'})
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func stripComment(line string) string {
	quote := byte(0)
	escaped := false
	for index := 0; index < len(line); index++ {
		character := line[index]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if quote == '"' && character == '\\' {
				escaped = true
				continue
			}
			if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
		case '#':
			return line[:index]
		}
	}
	return line
}

func parseString(value string) (string, error) {
	parsed, err := strconv.Unquote(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("expected quoted string: %w", err)
	}
	return parsed, nil
}

func parseStrings(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '[' || value[len(value)-1] != ']' {
		return nil, errors.New("expected an array of quoted strings")
	}
	value = strings.TrimSpace(value[1 : len(value)-1])
	if value == "" {
		return nil, nil
	}
	var result []string
	for value != "" {
		value = strings.TrimSpace(value)
		if value == "" || value[0] != '"' {
			return nil, errors.New("expected quoted array value")
		}
		end := 1
		escaped := false
		for ; end < len(value); end++ {
			if escaped {
				escaped = false
				continue
			}
			if value[end] == '\\' {
				escaped = true
				continue
			}
			if value[end] == '"' {
				break
			}
		}
		if end == len(value) {
			return nil, errors.New("unterminated array value")
		}
		item, err := strconv.Unquote(value[:end+1])
		if err != nil {
			return nil, err
		}
		result = append(result, item)
		value = strings.TrimSpace(value[end+1:])
		if value == "" {
			break
		}
		if value[0] != ',' {
			return nil, errors.New("expected comma between array values")
		}
		value = value[1:]
	}
	return result, nil
}
