package upgradeservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
)

// The complete tag diff records which owner applies each changed template
// file. Skills and named source migrations own executable changes; module
// operations use the Go command. Project-specific documentation stays manual.
type sampleReleaseManifest struct {
	From           string              `json:"from"`
	To             string              `json:"to"`
	PreviousCommit string              `json:"previous_commit"`
	TargetCommit   string              `json:"target_commit"`
	Files          []sampleReleaseFile `json:"files"`
}

type sampleReleaseFile struct {
	Path     string              `json:"path"`
	Action   string              `json:"action"`
	Handling string              `json:"handling"`
	Previous *upgradeFileContent `json:"previous,omitempty"`
	Target   *upgradeFileContent `json:"target,omitempty"`
}

func readSampleReleaseManifest(version string) (sampleReleaseManifest, bool, error) {
	data, err := skillsManifestFiles.ReadFile("manifests/" + version + "/sample-files.json")
	if errors.Is(err, fs.ErrNotExist) {
		return sampleReleaseManifest{}, false, nil // Finalized after the sample tag.
	}
	if err != nil {
		return sampleReleaseManifest{}, false, err
	}
	var manifest sampleReleaseManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, true, err
	}
	return manifest, true, validateSampleReleaseManifest(manifest)
}

func validateSampleReleaseManifest(manifest sampleReleaseManifest) error {
	if manifest.From != "v0.1.22" || manifest.To != "v0.1.23" || len(manifest.PreviousCommit) != 40 || len(manifest.TargetCommit) != 40 {
		return fmt.Errorf("invalid sample release manifest version or commits")
	}
	seen := make(map[string]bool)
	for _, file := range manifest.Files {
		path, err := cleanManifestPath(file.Path)
		if err != nil || path != file.Path || seen[path] {
			return fmt.Errorf("invalid or repeated sample path %q", file.Path)
		}
		seen[path] = true
		switch file.Handling {
		case "skills", "source-migration", "module", "manual-documentation":
		default:
			return fmt.Errorf("unowned sample change %s", path)
		}
		if (file.Action == "add" && (file.Previous != nil || file.Target == nil)) ||
			(file.Action == "update" && (file.Previous == nil || file.Target == nil)) ||
			(file.Action == "delete" && (file.Previous == nil || file.Target != nil)) {
			return fmt.Errorf("incomplete sample operation %s", path)
		}
		if file.Action != "add" && file.Action != "update" && file.Action != "delete" {
			return fmt.Errorf("invalid sample operation %s", path)
		}
		for _, content := range []*upgradeFileContent{file.Previous, file.Target} {
			if content != nil && sha256Hex([]byte(content.Content)) != content.SHA256 {
				return fmt.Errorf("sample manifest hash mismatch: %s", path)
			}
		}
	}
	return nil
}

func (e stepExecutor) checkSampleReleaseManifest() error {
	manifest, found, err := readSampleReleaseManifest(e.to)
	if err != nil {
		return err
	}
	if found {
		fmt.Fprintf(e.stdout, "  verified sample tag manifest: %d changed files\n", len(manifest.Files))
		for _, file := range manifest.Files {
			if file.Handling == "manual-documentation" {
				fmt.Fprintf(e.stdout, "  keep project-specific %s; merge template guidance from sample_app %s\n", file.Path, manifest.To)
			}
		}
	}
	return nil
}
