package jscommand

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

const DefaultEntrypointGroup = "default"

type Manifest struct {
	Package     string
	Output      OutputConfig
	Bundle      BundleConfig
	Entrypoints []Entrypoint
}

type OutputConfig struct {
	Dir        string
	PublicPath string
	Importmap  string
}

type BundleConfig struct {
	Shared    bool
	Minify    bool
	Sourcemap bool
	Target    string
}

type Entrypoint struct {
	Name       string
	Group      string
	Module     string
	Imports    []string
	ExtraFiles []string
	Assets     []string
}

func LoadManifest(root string) (Manifest, error) {
	path := filepath.Join(root, "js.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read js.toml: %w", err)
	}

	manifest, err := ParseManifest(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse js.toml: %w", err)
	}
	return manifest, nil
}

func ParseManifest(data []byte) (Manifest, error) {
	manifest := defaultManifest()
	lines := strings.Split(string(data), "\n")

	section := ""
	entryByName := map[string]int{}
	for index := 0; index < len(lines); index++ {
		lineNumber := index + 1
		line := strings.TrimSpace(stripComment(lines[index]))
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return Manifest{}, fmt.Errorf("line %d: malformed section header", lineNumber)
			}
			section = strings.TrimSpace(line[1 : len(line)-1])
			if strings.HasPrefix(section, "entrypoint.") {
				name, err := parseTableName(strings.TrimPrefix(section, "entrypoint."))
				if err != nil {
					return Manifest{}, fmt.Errorf("line %d: %w", lineNumber, err)
				}
				if name == "" {
					return Manifest{}, fmt.Errorf("line %d: entrypoint name is required", lineNumber)
				}
				if _, ok := entryByName[name]; !ok {
					manifest.Entrypoints = append(manifest.Entrypoints, Entrypoint{
						Name:  name,
						Group: DefaultEntrypointGroup,
					})
					entryByName[name] = len(manifest.Entrypoints) - 1
				}
			}
			continue
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return Manifest{}, fmt.Errorf("line %d: expected key = value", lineNumber)
		}
		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)
		if strings.HasPrefix(rawValue, "[") && !arrayClosed(rawValue) {
			var builder strings.Builder
			builder.WriteString(rawValue)
			for !arrayClosed(builder.String()) {
				index++
				if index >= len(lines) {
					return Manifest{}, fmt.Errorf("line %d: unterminated array", lineNumber)
				}
				builder.WriteByte('\n')
				builder.WriteString(strings.TrimSpace(stripComment(lines[index])))
			}
			rawValue = builder.String()
		}

		if err := assignManifestValue(&manifest, entryByName, section, key, rawValue); err != nil {
			return Manifest{}, fmt.Errorf("line %d: %w", lineNumber, err)
		}
	}

	manifest = normalizeManifest(manifest)
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func FormatManifest(manifest Manifest) ([]byte, error) {
	manifest = normalizeManifest(manifest)
	if err := ValidateManifest(manifest); err != nil {
		return nil, err
	}

	defaults := defaultManifest()
	var builder strings.Builder
	if manifest.Package != defaults.Package {
		writeString(&builder, "package", manifest.Package)
	}

	if manifest.Output != defaults.Output {
		writeSectionBreak(&builder)
		builder.WriteString("[output]\n")
		if manifest.Output.Dir != defaults.Output.Dir {
			writeString(&builder, "dir", manifest.Output.Dir)
		}
		if manifest.Output.PublicPath != defaults.Output.PublicPath {
			writeString(&builder, "public_path", manifest.Output.PublicPath)
		}
		if manifest.Output.Importmap != defaults.Output.Importmap {
			writeString(&builder, "importmap", manifest.Output.Importmap)
		}
	}

	if manifest.Bundle != defaults.Bundle {
		writeSectionBreak(&builder)
		builder.WriteString("[bundle]\n")
		if manifest.Bundle.Shared != defaults.Bundle.Shared {
			writeBool(&builder, "shared", manifest.Bundle.Shared)
		}
		if manifest.Bundle.Minify != defaults.Bundle.Minify {
			writeBool(&builder, "minify", manifest.Bundle.Minify)
		}
		if manifest.Bundle.Sourcemap != defaults.Bundle.Sourcemap {
			writeBool(&builder, "sourcemap", manifest.Bundle.Sourcemap)
		}
		if manifest.Bundle.Target != defaults.Bundle.Target {
			writeString(&builder, "target", manifest.Bundle.Target)
		}
	}

	for _, entrypoint := range manifest.Entrypoints {
		writeSectionBreak(&builder)
		fmt.Fprintf(&builder, "[entrypoint.%s]\n", formatTableName(entrypoint.Name))
		if entrypoint.Group != DefaultEntrypointGroup {
			writeString(&builder, "group", entrypoint.Group)
		}
		writeString(&builder, "module", entrypoint.Module)
		writeStringArray(&builder, "imports", entrypoint.Imports)
		writeStringArray(&builder, "extra_files", entrypoint.ExtraFiles)
		writeStringArray(&builder, "assets", entrypoint.Assets)
	}

	return []byte(builder.String()), nil
}

func ValidateManifest(manifest Manifest) error {
	if len(manifest.Entrypoints) == 0 {
		return fmt.Errorf("at least one [entrypoint.<name>] block is required")
	}
	seen := map[string]bool{}
	for _, entrypoint := range manifest.Entrypoints {
		if strings.TrimSpace(entrypoint.Name) == "" {
			return fmt.Errorf("entrypoint name is required")
		}
		if seen[entrypoint.Name] {
			return fmt.Errorf("entrypoint %q is already declared", entrypoint.Name)
		}
		seen[entrypoint.Name] = true
		if strings.TrimSpace(entrypoint.Module) == "" {
			return fmt.Errorf("entrypoint %q is missing module", entrypoint.Name)
		}
	}
	return nil
}

func normalizeManifest(manifest Manifest) Manifest {
	defaults := defaultManifest()
	if manifest.Package == "" {
		manifest.Package = defaults.Package
	}
	if manifest.Output.Dir == "" {
		manifest.Output.Dir = defaults.Output.Dir
	}
	if manifest.Output.PublicPath == "" {
		manifest.Output.PublicPath = defaults.Output.PublicPath
	}
	if manifest.Output.Importmap == "" {
		manifest.Output.Importmap = defaults.Output.Importmap
	}
	if manifest.Bundle.Target == "" {
		manifest.Bundle.Target = defaults.Bundle.Target
	}
	for index := range manifest.Entrypoints {
		if strings.TrimSpace(manifest.Entrypoints[index].Group) == "" {
			manifest.Entrypoints[index].Group = DefaultEntrypointGroup
		}
	}
	return manifest
}

func defaultManifest() Manifest {
	return Manifest{
		Package: "package.json",
		Output: OutputConfig{
			Dir:        "app/public/assets/lazyshaft",
			PublicPath: "/assets/lazyshaft",
			Importmap:  "app/public/assets/importmap.json",
		},
		Bundle: BundleConfig{
			Shared: true,
			Minify: true,
			Target: "es2020",
		},
	}
}

func assignManifestValue(manifest *Manifest, entries map[string]int, section, key, rawValue string) error {
	switch {
	case section == "":
		switch key {
		case "package":
			value, err := parseString(rawValue)
			if err != nil {
				return err
			}
			manifest.Package = value
		default:
			return fmt.Errorf("unknown key %q", key)
		}
	case section == "output":
		value, err := parseString(rawValue)
		if err != nil {
			return err
		}
		switch key {
		case "dir":
			manifest.Output.Dir = value
		case "public_path":
			manifest.Output.PublicPath = value
		case "importmap":
			manifest.Output.Importmap = value
		default:
			return fmt.Errorf("unknown output key %q", key)
		}
	case section == "bundle":
		switch key {
		case "shared":
			value, err := parseBool(rawValue)
			if err != nil {
				return err
			}
			manifest.Bundle.Shared = value
		case "minify":
			value, err := parseBool(rawValue)
			if err != nil {
				return err
			}
			manifest.Bundle.Minify = value
		case "sourcemap":
			value, err := parseBool(rawValue)
			if err != nil {
				return err
			}
			manifest.Bundle.Sourcemap = value
		case "target":
			value, err := parseString(rawValue)
			if err != nil {
				return err
			}
			manifest.Bundle.Target = value
		default:
			return fmt.Errorf("unknown bundle key %q", key)
		}
	case strings.HasPrefix(section, "entrypoint."):
		name, err := parseTableName(strings.TrimPrefix(section, "entrypoint."))
		if err != nil {
			return err
		}
		entrypointIndex, ok := entries[name]
		if !ok {
			return fmt.Errorf("entrypoint %q is not declared", name)
		}
		entrypoint := &manifest.Entrypoints[entrypointIndex]
		switch key {
		case "group":
			value, err := parseString(rawValue)
			if err != nil {
				return err
			}
			entrypoint.Group = value
		case "module":
			value, err := parseString(rawValue)
			if err != nil {
				return err
			}
			entrypoint.Module = value
		case "imports":
			value, err := parseStringArray(rawValue)
			if err != nil {
				return err
			}
			entrypoint.Imports = value
		case "extra_files", "workers":
			value, err := parseStringArray(rawValue)
			if err != nil {
				return err
			}
			entrypoint.ExtraFiles = append(entrypoint.ExtraFiles, value...)
		case "assets":
			value, err := parseStringArray(rawValue)
			if err != nil {
				return err
			}
			entrypoint.Assets = value
		default:
			return fmt.Errorf("unknown entrypoint key %q", key)
		}
	default:
		return fmt.Errorf("unknown section %q", section)
	}
	return nil
}

func writeSectionBreak(builder *strings.Builder) {
	if builder.Len() != 0 {
		builder.WriteByte('\n')
	}
}

func writeString(builder *strings.Builder, key, value string) {
	fmt.Fprintf(builder, "%s = %s\n", key, strconv.Quote(value))
}

func writeBool(builder *strings.Builder, key string, value bool) {
	fmt.Fprintf(builder, "%s = %t\n", key, value)
}

func writeStringArray(builder *strings.Builder, key string, values []string) {
	if len(values) == 0 {
		return
	}
	if len(values) == 1 {
		fmt.Fprintf(builder, "%s = [%s]\n", key, strconv.Quote(values[0]))
		return
	}
	fmt.Fprintf(builder, "%s = [\n", key)
	for _, value := range values {
		fmt.Fprintf(builder, "  %s,\n", strconv.Quote(value))
	}
	builder.WriteString("]\n")
}

func formatTableName(name string) string {
	if isBareTableName(name) {
		return name
	}
	return strconv.Quote(name)
}

func isBareTableName(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func stripComment(line string) string {
	inQuote := rune(0)
	escaped := false
	for index, char := range line {
		if escaped {
			escaped = false
			continue
		}
		if inQuote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if inQuote != 0 {
			if char == inQuote {
				inQuote = 0
			}
			continue
		}
		if char == '"' || char == '\'' {
			inQuote = char
			continue
		}
		if char == '#' {
			return line[:index]
		}
	}
	return line
}

func parseTableName(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "\"") || strings.HasPrefix(raw, "'") {
		return parseString(raw)
	}
	return raw, nil
}

func parseString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("string value is required")
	}
	if strings.HasPrefix(raw, "\"") {
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string: %w", err)
		}
		return value, nil
	}
	if strings.HasPrefix(raw, "'") {
		if !strings.HasSuffix(raw, "'") || len(raw) < 2 {
			return "", fmt.Errorf("invalid quoted string")
		}
		return raw[1 : len(raw)-1], nil
	}
	return raw, nil
}

func parseBool(raw string) (bool, error) {
	switch strings.TrimSpace(raw) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("expected true or false")
	}
}

func parseStringArray(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, fmt.Errorf("expected string array")
	}
	raw = strings.TrimSpace(raw[1 : len(raw)-1])
	if raw == "" {
		return nil, nil
	}

	parts, err := splitArrayItems(raw)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, err := parseString(part)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func splitArrayItems(raw string) ([]string, error) {
	var parts []string
	start := 0
	inQuote := rune(0)
	escaped := false
	for index, char := range raw {
		if escaped {
			escaped = false
			continue
		}
		if inQuote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if inQuote != 0 {
			if char == inQuote {
				inQuote = 0
			}
			continue
		}
		if char == '"' || char == '\'' {
			inQuote = char
			continue
		}
		if char == ',' {
			parts = append(parts, raw[start:index])
			start = index + 1
		}
	}
	if inQuote != 0 {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	parts = append(parts, raw[start:])
	return parts, nil
}

func arrayClosed(raw string) bool {
	inQuote := rune(0)
	escaped := false
	depth := 0
	for _, char := range raw {
		if escaped {
			escaped = false
			continue
		}
		if inQuote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if inQuote != 0 {
			if char == inQuote {
				inQuote = 0
			}
			continue
		}
		if char == '"' || char == '\'' {
			inQuote = char
			continue
		}
		if char == '[' {
			depth++
		}
		if char == ']' {
			depth--
		}
	}
	return depth <= 0
}
