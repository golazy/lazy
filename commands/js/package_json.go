package jscommand

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ensurePackageDependencies(path string, packages []string) (bool, error) {
	document, err := readPackageJSON(path)
	if err != nil {
		return false, err
	}

	changed := false
	if _, ok := document["private"]; !ok {
		document["private"] = true
		changed = true
	}

	dependencies, err := objectField(document, "dependencies")
	if err != nil {
		return false, err
	}
	if dependencies == nil {
		dependencies = map[string]any{}
		document["dependencies"] = dependencies
		changed = true
	}

	devDependencies, err := objectField(document, "devDependencies")
	if err != nil {
		return false, err
	}

	for _, name := range packages {
		if name == "" {
			continue
		}
		if _, ok := dependencies[name]; ok {
			continue
		}
		if devDependencies != nil {
			if _, ok := devDependencies[name]; ok {
				continue
			}
		}
		dependencies[name] = "latest"
		changed = true
	}

	if !changed {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create package.json directory: %w", err)
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal package.json: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return false, fmt.Errorf("write package.json: %w", err)
	}
	return true, nil
}

func readPackageJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}
	return document, nil
}

func objectField(document map[string]any, name string) (map[string]any, error) {
	value, ok := document[name]
	if !ok {
		return nil, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("package.json field %q must be an object", name)
	}
	return object, nil
}

func requiredPackages(manifest Manifest) []string {
	seen := map[string]bool{}
	for _, entrypoint := range manifest.Entrypoints {
		for _, specifier := range append([]string{entrypoint.Module}, entrypoint.ExtraFiles...) {
			pkg := packageName(specifier)
			if pkg != "" {
				seen[pkg] = true
			}
		}
	}

	packages := make([]string, 0, len(seen))
	for name := range seen {
		packages = append(packages, name)
	}
	sort.Strings(packages)
	return packages
}

func packageName(specifier string) string {
	specifier = strings.TrimSpace(specifier)
	if specifier == "" ||
		strings.HasPrefix(specifier, ".") ||
		strings.HasPrefix(specifier, "/") {
		return ""
	}

	parts := strings.Split(specifier, "/")
	if strings.HasPrefix(specifier, "@") {
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return ""
		}
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}
