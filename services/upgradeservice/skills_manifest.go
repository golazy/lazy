package upgradeservice

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"sort"
	"strings"
)

const (
	v021SkillsPreviousRoot = "manifests/v0.1.21/skills/previous"
	v021SkillsTargetRoot   = "manifests/v0.1.21/skills/target"
)

//go:embed all:manifests
var skillsManifestFiles embed.FS

func currentSkillsManifest(version string) (upgradeFileManifest, bool, error) {
	index := slices.Index(releaseVersions, version)
	if index <= 0 {
		return upgradeFileManifest{}, false, nil
	}
	previous := releaseVersions[index-1]
	previousRoot := fmt.Sprintf("manifests/%s/skills/previous", version)
	targetRoot := fmt.Sprintf("manifests/%s/skills/target", version)
	if _, err := fs.Stat(skillsManifestFiles, targetRoot); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return upgradeFileManifest{}, false, nil
		}
		return upgradeFileManifest{}, false, fmt.Errorf("inspect skill manifest for %s: %w", version, err)
	}
	manifest, err := buildSkillsManifest(previous, version, previousRoot, targetRoot)
	return manifest, true, err
}

func buildSkillsManifest(from string, to string, previousRoot string, targetRoot string) (upgradeFileManifest, error) {
	previous, err := readSkillsManifestTree(previousRoot)
	if err != nil {
		return upgradeFileManifest{}, err
	}
	target, err := readSkillsManifestTree(targetRoot)
	if err != nil {
		return upgradeFileManifest{}, err
	}

	paths := make(map[string]struct{}, len(previous)+len(target))
	for name := range previous {
		paths[name] = struct{}{}
	}
	for name := range target {
		paths[name] = struct{}{}
	}
	ordered := make([]string, 0, len(paths))
	for name := range paths {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)

	manifest := upgradeFileManifest{From: from, To: to}
	for _, name := range ordered {
		previousContent, hadPrevious := previous[name]
		targetContent, hasTarget := target[name]
		switch {
		case hadPrevious && hasTarget:
			action := upgradeFileUpdate
			if previousContent == targetContent {
				action = upgradeFileEnsure
			}
			manifest.Files = append(manifest.Files, upgradeFileOperation{
				Action:   action,
				Path:     name,
				Previous: upgradeManifestContent(previousContent),
				Target:   upgradeManifestContent(targetContent),
			})
		case hadPrevious && !hasTarget:
			manifest.Files = append(manifest.Files, upgradeFileOperation{
				Action:   upgradeFileDelete,
				Path:     name,
				Previous: upgradeManifestContent(previousContent),
			})
		case !hadPrevious && hasTarget:
			manifest.Files = append(manifest.Files, upgradeFileOperation{
				Action: upgradeFileAdd,
				Path:   name,
				Target: upgradeManifestContent(targetContent),
			})
		}
	}
	return manifest, nil
}

func readSkillsManifestTree(root string) (map[string]string, error) {
	files := make(map[string]string)
	err := fs.WalkDir(skillsManifestFiles, root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(name, root+"/")
		if relative == name || relative == "" {
			return fmt.Errorf("skill manifest path %q is outside %q", name, root)
		}
		data, err := skillsManifestFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read embedded skill manifest %s: %w", name, err)
		}
		files[".skills/"+relative] = string(data)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read skill manifest tree %s: %w", root, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("skill manifest tree %s is empty", root)
	}
	return files, nil
}

func (e stepExecutor) applySkillsManifest(manifest upgradeFileManifest) error {
	// Skills were not present in every historical app. Missing update targets
	// are therefore installed like additions, while customized files still use
	// the normal manifest conflict and backup flow.
	e.bootstrapFromOlder = true
	return e.applyFileManifest(manifest)
}
