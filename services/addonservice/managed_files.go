package addonservice

import (
	"fmt"
	"sort"
)

// mergeContributionFiles combines independent property-level rewrites of one
// application file. Whole-file generators and copied artifacts remain
// exclusive because composing their bytes would make ownership ambiguous.
func mergeContributionFiles(files []ManagedFile) ([]ManagedFile, error) {
	byPath := make(map[string][]ManagedFile, len(files))
	for _, file := range files {
		byPath[file.Path] = append(byPath[file.Path], file)
	}
	paths := make([]string, 0, len(byPath))
	for name := range byPath {
		paths = append(paths, name)
	}
	sort.Strings(paths)

	merged := make([]ManagedFile, 0, len(paths))
	for _, name := range paths {
		parts := byPath[name]
		if len(parts) == 1 {
			merged = append(merged, parts[0])
			continue
		}
		rewriteCount := 0
		sourceEditCount := 0
		combined := ManagedFile{Path: name, Kind: "composite"}
		for _, part := range parts {
			switch part.Kind {
			case "rewrite":
				rewriteCount++
			case "source-edit", "source-edit-created":
				sourceEditCount++
			default:
				return nil, fmt.Errorf("managed file %s has incompatible %s and property-level contributions", name, part.Kind)
			}
			if part.Mode != 0 {
				if combined.Mode != 0 && combined.Mode != part.Mode {
					return nil, fmt.Errorf("managed file %s has conflicting modes %o and %o", name, combined.Mode, part.Mode)
				}
				combined.Mode = part.Mode
			}
			combined.Owners = append(combined.Owners, part.Owners...)
			combined.Properties = append(combined.Properties, part.Properties...)
		}
		if rewriteCount != 1 || sourceEditCount != 1 || len(parts) != 2 {
			return nil, fmt.Errorf("managed file %s has ambiguous property-level contributors", name)
		}
		combined.Owners = normalizedStrings(combined.Owners)
		combined.Properties = normalizedStrings(combined.Properties)
		merged = append(merged, combined)
	}
	return merged, nil
}
