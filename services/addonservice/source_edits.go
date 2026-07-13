package addonservice

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"path"
	"sort"
	"strings"

	"golang.org/x/mod/module"
	"golazy.dev/lazycode"
	"golazy.dev/lazycode/gocode"
	"golazy.dev/lazycode/tomlcode"
)

const (
	sourceEditSchema         = 1
	sourceEditPropertyPrefix = "golazy-source-edit-v1:"
	maxSourceEditBytes       = 1 << 20
	maxSourceEditOperations  = 1024
	tomlSetStringV1          = "toml.set_string.v1"
	tomlSetStringsV1         = "toml.set_strings.v1"
	goEnsureImportV1         = "go.ensure_import.v1"
)

type sourceEditDescriptor struct {
	Schema     int               `json:"schema"`
	Operations []json.RawMessage `json:"operations"`
}

type sourceEditOperation struct {
	Kind     string
	Path     string
	Table    string
	Key      string
	Expected string
	Import   string
}

type sourceEditProperty struct {
	Schema       int      `json:"schema"`
	Kind         string   `json:"kind"`
	Table        string   `json:"table,omitempty"`
	Key          string   `json:"key,omitempty"`
	Import       string   `json:"import,omitempty"`
	Expected     string   `json:"expected"`
	Previous     *string  `json:"previous,omitempty"`
	TableExisted bool     `json:"table_existed,omitempty"`
	Owners       []string `json:"owners"`
}

type desiredSourceEdit struct {
	property sourceEditProperty
}

type sourceEditChange struct {
	property sourceEditProperty
	value    *string
}

func validateSourceEditReference(value string) error {
	if !validFilePath(value) {
		return fmt.Errorf("descriptor path must be application-relative")
	}
	if path.Ext(value) != ".json" {
		return fmt.Errorf("descriptor must be a .json file")
	}
	first, _, _ := strings.Cut(value, "/")
	if strings.EqualFold(first, ".git") || strings.EqualFold(first, ".secrets") {
		return fmt.Errorf("descriptor is outside the package artifact surface")
	}
	return nil
}

func parseSourceEditDescriptor(data []byte, name string) (sourceEditDescriptor, error) {
	if len(data) > maxSourceEditBytes {
		return sourceEditDescriptor{}, fmt.Errorf("descriptor exceeds %d bytes", maxSourceEditBytes)
	}
	var descriptor sourceEditDescriptor
	if err := decodeStrictJSON(data, &descriptor); err != nil {
		return sourceEditDescriptor{}, fmt.Errorf("parse descriptor: %w", err)
	}
	if descriptor.Schema != sourceEditSchema {
		return sourceEditDescriptor{}, fmt.Errorf("unsupported source edit schema %d", descriptor.Schema)
	}
	if len(descriptor.Operations) == 0 {
		return sourceEditDescriptor{}, errors.New("source edit descriptor must contain operations")
	}
	if len(descriptor.Operations) > maxSourceEditOperations {
		return sourceEditDescriptor{}, fmt.Errorf("descriptor exceeds %d operations", maxSourceEditOperations)
	}
	for index, raw := range descriptor.Operations {
		operation, err := parseSourceEditOperation(raw)
		if err != nil {
			return sourceEditDescriptor{}, fmt.Errorf("operation %d: %w", index+1, err)
		}
		encoded, err := json.Marshal(operation)
		if err != nil {
			return sourceEditDescriptor{}, err
		}
		descriptor.Operations[index] = encoded
	}
	_ = name
	return descriptor, nil
}

func parseSourceEditOperation(raw []byte) (sourceEditOperation, error) {
	var identity struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &identity); err != nil {
		return sourceEditOperation{}, fmt.Errorf("read kind: %w", err)
	}
	switch identity.Kind {
	case tomlSetStringV1:
		var declaration struct {
			Kind  string  `json:"kind"`
			Path  string  `json:"path"`
			Table string  `json:"table,omitempty"`
			Key   string  `json:"key"`
			Value *string `json:"value"`
		}
		if err := decodeStrictJSON(raw, &declaration); err != nil {
			return sourceEditOperation{}, err
		}
		if declaration.Value == nil {
			return sourceEditOperation{}, errors.New("value is required")
		}
		operation := sourceEditOperation{Kind: declaration.Kind, Path: declaration.Path, Table: declaration.Table, Key: declaration.Key, Expected: tomlcode.EncodeString(*declaration.Value)}
		if err := validateSourceEditOperation(operation); err != nil {
			return sourceEditOperation{}, err
		}
		return operation, nil
	case tomlSetStringsV1:
		var declaration struct {
			Kind   string    `json:"kind"`
			Path   string    `json:"path"`
			Table  string    `json:"table,omitempty"`
			Key    string    `json:"key"`
			Values *[]string `json:"values"`
		}
		if err := decodeStrictJSON(raw, &declaration); err != nil {
			return sourceEditOperation{}, err
		}
		if declaration.Values == nil {
			return sourceEditOperation{}, errors.New("values is required")
		}
		operation := sourceEditOperation{Kind: declaration.Kind, Path: declaration.Path, Table: declaration.Table, Key: declaration.Key, Expected: tomlcode.EncodeStrings(*declaration.Values)}
		if err := validateSourceEditOperation(operation); err != nil {
			return sourceEditOperation{}, err
		}
		return operation, nil
	case goEnsureImportV1:
		var declaration struct {
			Kind   string  `json:"kind"`
			Path   string  `json:"path"`
			Import string  `json:"import"`
			Name   *string `json:"name,omitempty"`
		}
		if err := decodeStrictJSON(raw, &declaration); err != nil {
			return sourceEditOperation{}, err
		}
		if declaration.Name == nil {
			return sourceEditOperation{}, errors.New("name is required")
		}
		operation := sourceEditOperation{Kind: declaration.Kind, Path: declaration.Path, Import: declaration.Import, Expected: *declaration.Name}
		if err := validateSourceEditOperation(operation); err != nil {
			return sourceEditOperation{}, err
		}
		return operation, nil
	default:
		if identity.Kind == "" {
			return sourceEditOperation{}, errors.New("kind is required")
		}
		return sourceEditOperation{}, fmt.Errorf("unsupported operation kind %q", identity.Kind)
	}
}

func decodeStrictJSON(data []byte, target any) error {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return errors.New("more than one JSON value")
	} else if err != io.EOF {
		return fmt.Errorf("trailing data: %w", err)
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var consume func() error
	consume = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return errors.New("JSON object key is not a string")
				}
				if seen[key] {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[key] = true
				if err := consume(); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil {
				return err
			}
			if end != json.Delim('}') {
				return errors.New("malformed JSON object")
			}
		case '[':
			for decoder.More() {
				if err := consume(); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil {
				return err
			}
			if end != json.Delim(']') {
				return errors.New("malformed JSON array")
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
		}
		return nil
	}
	if err := consume(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err == nil {
		return errors.New("more than one JSON value")
	} else if err != io.EOF {
		return fmt.Errorf("trailing data: %w", err)
	}
	return nil
}

func validateSourceEditOperation(operation sourceEditOperation) error {
	if err := validateSourceEditTarget(operation.Path); err != nil {
		return err
	}
	switch operation.Kind {
	case tomlSetStringV1, tomlSetStringsV1:
		if path.Ext(operation.Path) != ".toml" {
			return fmt.Errorf("%s target must end in .toml", operation.Kind)
		}
		document, _ := tomlcode.Parse(nil)
		if _, err := document.SetRaw(operation.Table, operation.Key, operation.Expected); err != nil {
			return fmt.Errorf("invalid TOML property: %w", err)
		}
	case goEnsureImportV1:
		if path.Ext(operation.Path) != ".go" {
			return fmt.Errorf("%s target must end in .go", operation.Kind)
		}
		if err := module.CheckImportPath(operation.Import); err != nil {
			return fmt.Errorf("invalid Go import %q: %w", operation.Import, err)
		}
		if operation.Expected != "_" && !token.IsIdentifier(operation.Expected) {
			return fmt.Errorf("Go import name %q is unsupported; use an explicit identifier or blank import", operation.Expected)
		}
	default:
		return fmt.Errorf("unsupported operation kind %q", operation.Kind)
	}
	return nil
}

func validateSourceEditTarget(name string) error {
	if !validFilePath(name) {
		return fmt.Errorf("source edit target %q is invalid", name)
	}
	for reserved := range reservedContributionTargets {
		if strings.EqualFold(name, reserved) {
			return fmt.Errorf("source edit target %q is installer-owned", name)
		}
	}
	first, _, _ := strings.Cut(name, "/")
	if strings.EqualFold(first, ".git") || strings.EqualFold(first, ".secrets") {
		return fmt.Errorf("source edit target %q is outside the application source surface", name)
	}
	return nil
}

func sourceEditTargetPaths(addons []InstalledAddon) ([]string, error) {
	var targets []string
	for _, addon := range addons {
		for _, artifact := range addon.EditArtifacts {
			descriptor, err := parseSourceEditDescriptor(artifact.Data, artifact.Source)
			if err != nil {
				return nil, fmt.Errorf("add-on %q source edit %q: %w", addon.ID, artifact.Source, err)
			}
			for _, raw := range descriptor.Operations {
				operation, err := decodeNormalizedSourceEditOperation(raw)
				if err != nil {
					return nil, err
				}
				targets = append(targets, operation.Path)
			}
		}
	}
	return normalizedStrings(targets), nil
}

func decodeNormalizedSourceEditOperation(raw []byte) (sourceEditOperation, error) {
	var operation sourceEditOperation
	if err := json.Unmarshal(raw, &operation); err != nil {
		return sourceEditOperation{}, err
	}
	return operation, validateSourceEditOperation(operation)
}

func sourceEditContributions(before State, addons []InstalledAddon, current map[string][]byte) ([]lazycode.Operation, []ManagedFile, error) {
	desired := map[string]map[string]desiredSourceEdit{}
	active := make(map[string]InstalledAddon, len(addons))
	loaded := map[string]bool{}
	for _, addon := range addons {
		active[addon.ID] = addon
		if len(addon.EditArtifacts) != 0 {
			loaded[addon.ID] = true
		}
		for _, artifact := range addon.EditArtifacts {
			descriptor, err := parseSourceEditDescriptor(artifact.Data, artifact.Source)
			if err != nil {
				return nil, nil, fmt.Errorf("add-on %q source edit %q: %w", addon.ID, artifact.Source, err)
			}
			for _, raw := range descriptor.Operations {
				operation, err := decodeNormalizedSourceEditOperation(raw)
				if err != nil {
					return nil, nil, err
				}
				property := propertyFromSourceEdit(operation, addon.ID)
				if err := mergeDesiredSourceEdit(desired, operation.Path, property); err != nil {
					return nil, nil, err
				}
			}
		}
	}

	// A locked source remains usable when its package manifest is temporarily
	// unavailable. Preserve only the properties owned by that still-selected
	// add-on; a loaded descriptor is authoritative and may remove old edits.
	for _, managed := range before.Managed {
		previous, err := decodeSourceEditProperties(managed.Properties)
		if err != nil {
			return nil, nil, fmt.Errorf("decode %s source edits: %w", managed.Path, err)
		}
		if len(previous) == 0 {
			continue
		}
		for _, property := range previous {
			for _, owner := range property.Owners {
				addon, selected := active[owner]
				if !selected || loaded[owner] || len(addon.SourceEdits) == 0 {
					continue
				}
				copy := property
				copy.Owners = []string{owner}
				if err := mergeDesiredSourceEdit(desired, managed.Path, copy); err != nil {
					return nil, nil, err
				}
			}
		}
	}

	paths := map[string]bool{}
	for name := range desired {
		paths[name] = true
	}
	for _, managed := range before.Managed {
		previous, err := decodeSourceEditProperties(managed.Properties)
		if err != nil {
			return nil, nil, fmt.Errorf("decode %s source edits: %w", managed.Path, err)
		}
		if len(previous) != 0 {
			paths[managed.Path] = true
		}
	}
	ordered := make([]string, 0, len(paths))
	for name := range paths {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)

	var operations []lazycode.Operation
	var managedFiles []ManagedFile
	for _, name := range ordered {
		operation, managed, err := reconcileSourceEditFile(name, before, current, desired[name])
		if err != nil {
			return nil, nil, err
		}
		if operation != nil {
			operations = append(operations, operation)
		}
		if managed != nil {
			managedFiles = append(managedFiles, *managed)
		}
	}
	return operations, managedFiles, nil
}

func propertyFromSourceEdit(operation sourceEditOperation, owner string) sourceEditProperty {
	return sourceEditProperty{
		Schema:   sourceEditSchema,
		Kind:     operation.Kind,
		Table:    operation.Table,
		Key:      operation.Key,
		Import:   operation.Import,
		Expected: operation.Expected,
		Owners:   []string{owner},
	}
}

func mergeDesiredSourceEdit(all map[string]map[string]desiredSourceEdit, name string, property sourceEditProperty) error {
	if all[name] == nil {
		all[name] = map[string]desiredSourceEdit{}
	}
	identity, err := property.identity()
	if err != nil {
		return err
	}
	entry, exists := all[name][identity]
	if exists && (entry.property.Kind != property.Kind || entry.property.Expected != property.Expected) {
		return fmt.Errorf("add-ons contribute incompatible source edits to %s property %s", name, identity)
	}
	if !exists {
		entry.property = property
	}
	entry.property.Owners = normalizedStrings(append(entry.property.Owners, property.Owners...))
	all[name][identity] = entry
	return nil
}

func reconcileSourceEditFile(name string, before State, current map[string][]byte, desired map[string]desiredSourceEdit) (lazycode.Operation, *ManagedFile, error) {
	previousFile, wasManaged := before.managedFile(name)
	previous, err := decodeSourceEditProperties(previousFile.Properties)
	if err != nil {
		return nil, nil, fmt.Errorf("decode %s source edits: %w", name, err)
	}
	source, exists := current[name]
	for identity, property := range previous {
		actual, found, err := sourceEditActual(name, source, exists, property)
		if err != nil {
			return nil, nil, err
		}
		if !found || actual != property.Expected {
			return nil, nil, fmt.Errorf("managed source edit %s property %s drifted from %q", name, identity, property.Expected)
		}
	}

	identities := map[string]bool{}
	for identity := range previous {
		identities[identity] = true
	}
	for identity := range desired {
		identities[identity] = true
	}
	ordered := make([]string, 0, len(identities))
	for identity := range identities {
		ordered = append(ordered, identity)
	}
	sort.Strings(ordered)

	var records []sourceEditProperty
	var changes []sourceEditChange
	var owners []string
	for _, identity := range ordered {
		want, wanted := desired[identity]
		old, tracked := previous[identity]
		if wanted {
			property := want.property
			if tracked {
				property.Previous = old.Previous
				property.TableExisted = old.TableExisted
			} else {
				actual, found, err := sourceEditActual(name, source, exists, property)
				if err != nil {
					return nil, nil, err
				}
				// A TOML set operation explicitly owns and replaces this one
				// property, so retain any prior value for uninstall. Go imports
				// are structural identities; silently changing an existing alias
				// can change program meaning and is therefore a conflict.
				if found && actual != property.Expected && property.Kind == goEnsureImportV1 {
					return nil, nil, fmt.Errorf("source edit target %s property %s already has incompatible value %q", name, identity, actual)
				}
				if found {
					property.Previous = stringPointer(actual)
				}
				if property.Kind == tomlSetStringV1 || property.Kind == tomlSetStringsV1 {
					document, err := tomlcode.Parse(source)
					if err != nil {
						return nil, nil, err
					}
					property.TableExisted, err = document.HasTable(property.Table)
					if err != nil {
						return nil, nil, err
					}
				}
			}
			property.Owners = normalizedStrings(property.Owners)
			value := property.Expected
			changes = append(changes, sourceEditChange{property: property, value: &value})
			records = append(records, property)
			owners = append(owners, property.Owners...)
			continue
		}
		changes = append(changes, sourceEditChange{property: old, value: old.Previous})
	}

	removeWholeFile := len(records) == 0 && wasManaged && previousFile.Kind == "source-edit-created" && exists && lazycode.Hash(source) == previousFile.Baseline
	operation, err := buildSourceEditOperation(name, source, exists, changes, removeWholeFile)
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return operation, nil, nil
	}
	kind := "source-edit"
	if len(previous) != 0 && previousFile.Kind == "source-edit-created" {
		kind = "source-edit-created"
	} else if !wasManaged && !exists {
		kind = "source-edit-created"
	}
	managed := &ManagedFile{
		Path:       name,
		Kind:       kind,
		Owners:     normalizedStrings(owners),
		Properties: encodeSourceEditProperties(records),
	}
	return operation, managed, nil
}

func sourceEditActual(name string, source []byte, exists bool, property sourceEditProperty) (string, bool, error) {
	switch property.Kind {
	case tomlSetStringV1, tomlSetStringsV1:
		document, err := tomlcode.Parse(source)
		if err != nil {
			return "", false, fmt.Errorf("parse source edit target %s: %w", name, err)
		}
		value, found, err := document.Raw(property.Table, property.Key)
		if err != nil {
			return "", false, fmt.Errorf("read source edit target %s: %w", name, err)
		}
		return value, found, nil
	case goEnsureImportV1:
		if !exists {
			return "", false, nil
		}
		_, file, err := gocode.Parse(name, source)
		if err != nil {
			return "", false, err
		}
		return gocode.ImportName(file, property.Import)
	default:
		return "", false, fmt.Errorf("unsupported source edit property kind %q", property.Kind)
	}
}

func buildSourceEditOperation(name string, source []byte, exists bool, changes []sourceEditChange, removeWholeFile bool) (lazycode.Operation, error) {
	if len(changes) == 0 {
		return nil, nil
	}
	if removeWholeFile {
		return lazycode.OperationFunc(func(workspace *lazycode.Workspace) error { return workspace.Remove(name) }), nil
	}
	switch path.Ext(name) {
	case ".toml":
		// Validate against the same source used for property drift checks before
		// returning a lazycode operation for the transaction.
		if _, err := applyTOMLSourceEdits(source, changes); err != nil {
			return nil, err
		}
		return lazycode.OperationFunc(func(workspace *lazycode.Workspace) error {
			data := []byte(nil)
			if workspace.Exists(name) {
				var err error
				data, err = workspace.Read(name)
				if err != nil {
					return err
				}
			}
			result, err := applyTOMLSourceEdits(data, changes)
			if err != nil {
				return err
			}
			return workspace.Replace(name, result)
		}), nil
	case ".go":
		if !exists {
			return nil, fmt.Errorf("Go source edit target %s does not exist", name)
		}
		return gocode.Rewrite(name, func(_ *token.FileSet, file *ast.File) (bool, error) {
			changed := false
			for _, change := range changes {
				actual, found, err := gocode.ImportName(file, change.property.Import)
				if err != nil {
					return false, err
				}
				if change.value == nil {
					if found {
						if change.property.Expected != "_" && gocode.UsesSelector(file, change.property.Expected) {
							return false, fmt.Errorf("source edit target %s still uses import name %q; remove those references before uninstalling the add-on", name, change.property.Expected)
						}
						changed = gocode.RemoveImport(file, change.property.Import) || changed
					}
					continue
				}
				if found && actual == *change.value {
					continue
				}
				if found {
					changed = gocode.RemoveImport(file, change.property.Import) || changed
				}
				added, err := gocode.EnsureNamedImport(file, *change.value, change.property.Import)
				if err != nil {
					return false, err
				}
				changed = added || changed
			}
			return changed, nil
		}), nil
	default:
		return nil, fmt.Errorf("unsupported source edit target %s", name)
	}
}

// applyTOMLSourceEdits uses lazycode's conservative TOML document instead of
// text replacement, preserving unrelated keys and comments.
func applyTOMLSourceEdits(source []byte, changes []sourceEditChange) ([]byte, error) {
	document, err := tomlcode.Parse(source)
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		if change.value == nil {
			if _, err := document.Remove(change.property.Table, change.property.Key); err != nil {
				return nil, err
			}
			if !change.property.TableExisted {
				if _, err := document.RemoveTableIfEmpty(change.property.Table); err != nil {
					return nil, err
				}
			}
			continue
		}
		if _, err := document.SetRaw(change.property.Table, change.property.Key, *change.value); err != nil {
			return nil, err
		}
	}
	return document.Bytes(), nil
}

func (property sourceEditProperty) identity() (string, error) {
	if property.Schema != sourceEditSchema {
		return "", fmt.Errorf("unsupported source edit property schema %d", property.Schema)
	}
	switch property.Kind {
	case tomlSetStringV1, tomlSetStringsV1:
		if property.Key == "" || property.Import != "" {
			return "", errors.New("invalid TOML source edit property")
		}
		return "toml:" + property.Table + "\x00" + property.Key, nil
	case goEnsureImportV1:
		if property.Import == "" || property.Table != "" || property.Key != "" {
			return "", errors.New("invalid Go source edit property")
		}
		return "go-import:" + property.Import, nil
	default:
		return "", fmt.Errorf("unsupported source edit property kind %q", property.Kind)
	}
}

func encodeSourceEditProperties(properties []sourceEditProperty) []string {
	encoded := make([]string, 0, len(properties))
	for _, property := range properties {
		property.Owners = normalizedStrings(property.Owners)
		data, _ := json.Marshal(property)
		encoded = append(encoded, sourceEditPropertyPrefix+string(data))
	}
	sort.Strings(encoded)
	return encoded
}

func decodeSourceEditProperties(values []string) (map[string]sourceEditProperty, error) {
	properties := map[string]sourceEditProperty{}
	for _, value := range values {
		if !strings.HasPrefix(value, sourceEditPropertyPrefix) {
			continue
		}
		var property sourceEditProperty
		if err := decodeStrictJSON([]byte(strings.TrimPrefix(value, sourceEditPropertyPrefix)), &property); err != nil {
			return nil, err
		}
		identity, err := property.identity()
		if err != nil || len(property.Owners) == 0 {
			return nil, fmt.Errorf("invalid source edit property record %q", value)
		}
		if _, duplicate := properties[identity]; duplicate {
			return nil, fmt.Errorf("duplicate source edit property %s", identity)
		}
		property.Owners = normalizedStrings(property.Owners)
		properties[identity] = property
	}
	return properties, nil
}
