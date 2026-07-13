package addonservice

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func ReadState(root string) (State, error) {
	data, err := os.ReadFile(filepath.Join(root, StateFilename))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{Manifest: 1}, nil
		}
		return State{}, fmt.Errorf("read %s: %w", StateFilename, err)
	}
	state, err := ParseState(data)
	if err != nil {
		return State{}, fmt.Errorf("parse %s: %w", StateFilename, err)
	}
	return state, nil
}

func ParseState(data []byte) (State, error) {
	state := State{}
	section := ""
	addonIndex, moduleIndex, managedIndex := -1, -1, -1
	for lineIndex, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(stripComment(rawLine))
		if line == "" {
			continue
		}
		switch line {
		case "[[addons]]":
			state.Addons = append(state.Addons, InstalledAddon{})
			addonIndex = len(state.Addons) - 1
			moduleIndex, managedIndex = -1, -1
			section = "addons"
			continue
		case "[[modules]]":
			state.Modules = append(state.Modules, ManagedModule{})
			moduleIndex = len(state.Modules) - 1
			addonIndex, managedIndex = -1, -1
			section = "modules"
			continue
		case "[[managed]]":
			state.Managed = append(state.Managed, ManagedFile{})
			managedIndex = len(state.Managed) - 1
			addonIndex, moduleIndex = -1, -1
			section = "managed"
			continue
		}
		if strings.HasPrefix(line, "[") {
			return State{}, fmt.Errorf("line %d: unsupported table %q", lineIndex+1, line)
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return State{}, fmt.Errorf("line %d: expected key = value", lineIndex+1)
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		var err error
		switch section {
		case "":
			if key != "manifest" {
				return State{}, fmt.Errorf("line %d: unknown root key %q", lineIndex+1, key)
			}
			state.Manifest, err = strconv.Atoi(value)
		case "addons":
			err = parseAddonField(&state.Addons[addonIndex], key, value)
		case "modules":
			err = parseModuleField(&state.Modules[moduleIndex], key, value)
		case "managed":
			err = parseManagedField(&state.Managed[managedIndex], key, value)
		}
		if err != nil {
			return State{}, fmt.Errorf("line %d: %s: %w", lineIndex+1, key, err)
		}
	}
	if state.Manifest == 0 {
		return State{}, errors.New("manifest version is required")
	}
	if state.Manifest != 1 {
		return State{}, fmt.Errorf("unsupported manifest version %d", state.Manifest)
	}
	if err := validateState(state); err != nil {
		return State{}, err
	}
	normalizeState(&state)
	return state, nil
}

func parseAddonField(addon *InstalledAddon, key, value string) error {
	var err error
	switch key {
	case "id":
		addon.ID, err = parseString(value)
	case "version":
		addon.Version, err = parseString(value)
	case "direct":
		addon.Direct, err = strconv.ParseBool(value)
	case "config":
		var entries []string
		entries, err = parseStrings(value)
		if err == nil {
			addon.Config, err = parseConfigEntries(entries)
		}
	case "description":
		addon.Description, err = parseString(value)
	case "package":
		addon.Package, err = parseString(value)
	case "package_version":
		addon.PackageVersion, err = parseString(value)
	case "manifest_digest":
		addon.ManifestDigest, err = parseString(value)
	case "source":
		addon.Source, err = parseString(value)
	case "go_import":
		addon.GoImport, err = parseString(value)
	case "go_module":
		addon.GoModule, err = parseString(value)
	case "go_version":
		addon.GoModuleVersion, err = parseString(value)
	case "requires":
		addon.Requires, err = parseStrings(value)
	case "optional":
		addon.Optional, err = parseStrings(value)
	case "conflicts":
		addon.Conflicts, err = parseStrings(value)
	case "hooks":
		addon.Hooks, err = parseStrings(value)
	case "js_libraries":
		addon.JavaScript.Libraries, err = parseStrings(value)
	case "js_entrypoints":
		addon.JavaScript.Entrypoints, err = parseStrings(value)
	case "build_hooks":
		addon.Build.Hooks, err = parseStrings(value)
	case "tasks":
		addon.Build.Tasks, err = parseStrings(value)
	case "files":
		addon.Files, err = parseStrings(value)
	case "source_edits":
		addon.SourceEdits, err = parseStrings(value)
	default:
		return fmt.Errorf("unknown add-on key")
	}
	return err
}

func parseModuleField(module *ManagedModule, key, value string) error {
	var err error
	switch key {
	case "path":
		module.Path, err = parseString(value)
	case "version":
		module.Version, err = parseString(value)
	case "owners":
		module.Owners, err = parseStrings(value)
	case "managed":
		module.Managed, err = strconv.ParseBool(value)
	case "previous":
		module.Previous, err = parseString(value)
	case "previous_indirect":
		module.PreviousIndirect, err = strconv.ParseBool(value)
	default:
		return fmt.Errorf("unknown module key")
	}
	return err
}

func parseManagedField(file *ManagedFile, key, value string) error {
	var err error
	switch key {
	case "path":
		file.Path, err = parseString(value)
	case "kind":
		file.Kind, err = parseString(value)
	case "owners":
		file.Owners, err = parseStrings(value)
	case "baseline":
		file.Baseline, err = parseString(value)
	case "mode":
		var parsed uint64
		parsed, err = strconv.ParseUint(value, 10, 32)
		file.Mode = uint32(parsed)
	case "properties":
		file.Properties, err = parseStrings(value)
	default:
		return fmt.Errorf("unknown managed-file key")
	}
	return err
}

func validateState(state State) error {
	addonIDs := map[string]bool{}
	for _, addon := range state.Addons {
		normalizeResolved(&addon.ResolvedAddon)
		if err := validateResolved(addon.ResolvedAddon); err != nil {
			return err
		}
		if err := validateAddonConfig(addon.Config); err != nil {
			return fmt.Errorf("add-on %q: %w", addon.ID, err)
		}
		if addonIDs[addon.ID] {
			return fmt.Errorf("add-on %q is duplicated", addon.ID)
		}
		addonIDs[addon.ID] = true
	}
	modulePaths := map[string]bool{}
	for _, module := range state.Modules {
		if !validModuleVersion(module.Path, module.Version) {
			return fmt.Errorf("managed module must pin a path and exact version")
		}
		if modulePaths[module.Path] {
			return fmt.Errorf("managed module %q is duplicated", module.Path)
		}
		modulePaths[module.Path] = true
	}
	managedPaths := map[string]bool{}
	for _, file := range state.Managed {
		if !validFilePath(file.Path) || file.Kind == "" || file.Baseline == "" {
			return fmt.Errorf("managed file must declare path, kind, and baseline")
		}
		if managedPaths[file.Path] {
			return fmt.Errorf("managed file %q is duplicated", file.Path)
		}
		managedPaths[file.Path] = true
	}
	return nil
}

func MarshalState(state State) ([]byte, error) {
	normalizeState(&state)
	if err := validateState(state); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	fmt.Fprintf(&output, "manifest = %d\n", state.Manifest)
	for _, addon := range state.Addons {
		fmt.Fprintln(&output, "\n[[addons]]")
		writeString(&output, "id", addon.ID)
		writeString(&output, "version", addon.Version)
		fmt.Fprintf(&output, "direct = %t\n", addon.Direct)
		writeOptionalStrings(&output, "config", configEntries(addon.Config))
		writeOptionalString(&output, "description", addon.Description)
		writeString(&output, "package", addon.Package)
		writeString(&output, "package_version", addon.PackageVersion)
		writeString(&output, "manifest_digest", addon.ManifestDigest)
		writeString(&output, "source", addon.Source)
		writeString(&output, "go_import", addon.GoImport)
		writeString(&output, "go_module", addon.GoModule)
		writeString(&output, "go_version", addon.GoModuleVersion)
		writeOptionalStrings(&output, "requires", addon.Requires)
		writeOptionalStrings(&output, "optional", addon.Optional)
		writeOptionalStrings(&output, "conflicts", addon.Conflicts)
		writeOptionalStrings(&output, "hooks", addon.Hooks)
		writeOptionalStrings(&output, "js_libraries", addon.JavaScript.Libraries)
		writeOptionalStrings(&output, "js_entrypoints", addon.JavaScript.Entrypoints)
		writeOptionalStrings(&output, "build_hooks", addon.Build.Hooks)
		writeOptionalStrings(&output, "tasks", addon.Build.Tasks)
		writeOptionalStrings(&output, "files", addon.Files)
		writeOptionalStrings(&output, "source_edits", addon.SourceEdits)
	}
	for _, module := range state.Modules {
		fmt.Fprintln(&output, "\n[[modules]]")
		writeString(&output, "path", module.Path)
		writeString(&output, "version", module.Version)
		writeStrings(&output, "owners", module.Owners)
		fmt.Fprintf(&output, "managed = %t\n", module.Managed)
		writeString(&output, "previous", module.Previous)
		fmt.Fprintf(&output, "previous_indirect = %t\n", module.PreviousIndirect)
	}
	for _, file := range state.Managed {
		fmt.Fprintln(&output, "\n[[managed]]")
		writeString(&output, "path", file.Path)
		writeString(&output, "kind", file.Kind)
		writeStrings(&output, "owners", file.Owners)
		writeString(&output, "baseline", file.Baseline)
		if file.Mode != 0 {
			fmt.Fprintf(&output, "mode = %d\n", file.Mode)
		}
		writeOptionalStrings(&output, "properties", file.Properties)
	}
	return output.Bytes(), nil
}

func parseConfigEntries(entries []string) (map[string]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	config := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, found := strings.Cut(entry, "=")
		if !found || !validConfigKey(key) {
			return nil, fmt.Errorf("expected key=value with a valid configuration key")
		}
		if _, exists := config[key]; exists {
			return nil, fmt.Errorf("configuration key %q is duplicated", key)
		}
		config[key] = value
	}
	return config, nil
}

func configEntries(config map[string]string) []string {
	if len(config) == 0 {
		return nil
	}
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]string, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, key+"="+config[key])
	}
	return entries
}

func writeString(output *bytes.Buffer, key, value string) {
	fmt.Fprintf(output, "%s = %s\n", key, strconv.Quote(value))
}

func writeOptionalString(output *bytes.Buffer, key, value string) {
	if value != "" {
		writeString(output, key, value)
	}
}

func writeStrings(output *bytes.Buffer, key string, values []string) {
	values = append([]string(nil), values...)
	sort.Strings(values)
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = strconv.Quote(value)
	}
	fmt.Fprintf(output, "%s = [%s]\n", key, strings.Join(quoted, ", "))
}

func writeOptionalStrings(output *bytes.Buffer, key string, values []string) {
	if len(values) != 0 {
		writeStrings(output, key, values)
	}
}
