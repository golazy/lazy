package upgradeservice

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type upgradeMiseManifest struct {
	From           string
	To             string
	MissingContent string
	Tools          []upgradeMiseTool
	ObsoleteTasks  []upgradeMiseTask
}

type upgradeMiseTool struct {
	Name     string
	Previous string
	Target   string
	Reason   string
}

type upgradeMiseTask struct {
	Name        string
	Replacement string
}

type miseManifestResult struct {
	Data     []byte
	Changed  bool
	Messages []string
}

func upgradeTo011MiseManifest() upgradeMiseManifest {
	return upgradeMiseManifest{
		From:           "v0.1.10",
		To:             "v0.1.11",
		MissingContent: v011MiseToml,
		Tools: []upgradeMiseTool{
			{
				Name:     "go",
				Previous: "1.26.0",
				Reason:   "Go uses the go.mod go directive and toolchain selection",
			},
			{Name: "node", Target: "24"},
			{Name: "aqua:FiloSottile/age", Previous: "latest", Target: "1.3.1"},
			{Name: "aqua:getsops/sops", Previous: "latest", Target: "3.13.1"},
			{Name: "aqua:jdx/usage", Previous: "latest", Target: "3.5.3"},
		},
		ObsoleteTasks: []upgradeMiseTask{
			{Name: "tasks.dev", Replacement: ".mise/tasks/dev"},
			{Name: "tasks.test", Replacement: ".mise/tasks/test"},
		},
	}
}

func currentMiseCleanupManifest(version string) upgradeMiseManifest {
	return upgradeMiseManifest{
		To: version,
		Tools: []upgradeMiseTool{{
			Name:   "go",
			Reason: "Go uses the go.mod go directive and toolchain selection",
		}},
	}
}

func (e stepExecutor) applyMiseManifest(manifest upgradeMiseManifest) error {
	if manifest.From != "" && manifest.From != e.from {
		return fmt.Errorf("upgrade mise manifest starts at %s, want %s", manifest.From, e.from)
	}
	if manifest.To != "" && manifest.To != e.to {
		return fmt.Errorf("upgrade mise manifest targets %s, want %s", manifest.To, e.to)
	}
	path := filepath.Join(e.dir, "mise.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return e.createMissingMiseToml(path, manifest)
		}
		return fmt.Errorf("read mise.toml: %w", err)
	}
	result, err := updateMiseToml(data, manifest)
	if err != nil {
		return err
	}
	if !result.Changed {
		return nil
	}
	if e.dryRun {
		for _, message := range result.Messages {
			fmt.Fprintf(e.stdout, "  would %s\n", message)
		}
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect mise.toml: %w", err)
	}
	if err := os.WriteFile(path, result.Data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write mise.toml: %w", err)
	}
	for _, message := range result.Messages {
		fmt.Fprintf(e.stdout, "  %s\n", message)
	}
	return nil
}

func (e stepExecutor) createMissingMiseToml(path string, manifest upgradeMiseManifest) error {
	if strings.TrimSpace(manifest.MissingContent) == "" {
		return nil
	}
	if e.dryRun {
		fmt.Fprintln(e.stdout, "  would add mise.toml")
		return nil
	}
	if err := os.WriteFile(path, []byte(manifest.MissingContent), 0o644); err != nil {
		return fmt.Errorf("write mise.toml: %w", err)
	}
	fmt.Fprintln(e.stdout, "  added mise.toml")
	return nil
}

func updateMiseToml(data []byte, manifest upgradeMiseManifest) (miseManifestResult, error) {
	lines := splitPhysicalLines(data)
	if len(lines) == 0 {
		lines = []string{""}
	}
	result := miseManifestResult{Data: data}

	lines, changed, messages, err := updateMiseTools(lines, manifest)
	if err != nil {
		return result, err
	}
	result.Changed = result.Changed || changed
	result.Messages = append(result.Messages, messages...)

	lines, changed, messages = commentObsoleteMiseTasks(lines, manifest)
	result.Changed = result.Changed || changed
	result.Messages = append(result.Messages, messages...)

	if result.Changed {
		result.Data = []byte(strings.Join(lines, ""))
	}
	return result, nil
}

func updateMiseTools(lines []string, manifest upgradeMiseManifest) ([]string, bool, []string, error) {
	toolsByName := make(map[string]upgradeMiseTool)
	for _, tool := range manifest.Tools {
		if strings.TrimSpace(tool.Name) == "" {
			return nil, false, nil, fmt.Errorf("mise manifest tool name is required")
		}
		toolsByName[tool.Name] = tool
	}
	if len(toolsByName) == 0 {
		return lines, false, nil, nil
	}
	start, end := miseToolsSection(lines)
	if start == -1 {
		section := []string{"[tools]\n"}
		for _, tool := range manifest.Tools {
			if tool.Target == "" {
				continue
			}
			section = append(section, formatMiseToolLine(tool.Name, tool.Target))
		}
		if len(section) == 1 {
			return lines, false, nil, nil
		}
		next := append(section, "\n")
		next = append(next, lines...)
		return next, true, []string{"added mise.toml [tools] entries"}, nil
	}

	seen := make(map[string]bool)
	changed := false
	var messages []string
	section := slicesClone(lines[start+1 : end])
	for index, line := range section {
		key, value, ok := parseMiseToolLine(line)
		if !ok {
			continue
		}
		tool, managed := toolsByName[key]
		if !managed {
			continue
		}
		seen[key] = true
		switch {
		case tool.Target == "":
			section[index] = commentMiseToolLine(line, tool, manifest.To)
			changed = true
			messages = append(messages, fmt.Sprintf("marked mise tool %s as not needed", key))
		case value != tool.Target:
			section[index] = formatMiseToolLine(tool.Name, tool.Target)
			changed = true
			messages = append(messages, fmt.Sprintf("updated mise tool %s to %s", key, tool.Target))
		}
	}
	for _, tool := range manifest.Tools {
		if tool.Target == "" || seen[tool.Name] {
			continue
		}
		section = append(section, formatMiseToolLine(tool.Name, tool.Target))
		changed = true
		messages = append(messages, fmt.Sprintf("added mise tool %s %s", tool.Name, tool.Target))
	}
	if !changed {
		return lines, false, nil, nil
	}
	next := slicesClone(lines[:start+1])
	next = append(next, section...)
	next = append(next, lines[end:]...)
	return next, true, messages, nil
}

func commentObsoleteMiseTasks(lines []string, manifest upgradeMiseManifest) ([]string, bool, []string) {
	if len(manifest.ObsoleteTasks) == 0 {
		return lines, false, nil
	}
	tasks := make(map[string]upgradeMiseTask)
	for _, task := range manifest.ObsoleteTasks {
		tasks[task.Name] = task
	}
	changed := false
	var messages []string
	var out []string
	for index := 0; index < len(lines); {
		name, ok := miseTableHeader(strings.TrimSpace(stripMiseComment(lines[index])))
		task, obsolete := tasks[name]
		if !ok || !obsolete {
			out = append(out, lines[index])
			index++
			continue
		}
		reason := fmt.Sprintf("# GoLazy %s: [%s] is not needed; use %s.\n", manifest.To, task.Name, task.Replacement)
		out = append(out, reason)
		for index < len(lines) {
			out = append(out, commentTomlLine(lines[index]))
			index++
			if index < len(lines) {
				if _, nextTable := miseTableHeader(strings.TrimSpace(stripMiseComment(lines[index]))); nextTable {
					break
				}
			}
		}
		for index < len(lines) {
			if _, nextTable := miseTableHeader(strings.TrimSpace(stripMiseComment(lines[index]))); nextTable {
				break
			}
			out = append(out, commentTomlLine(lines[index]))
			index++
		}
		changed = true
		messages = append(messages, fmt.Sprintf("marked mise task %s as not needed", task.Name))
	}
	return out, changed, messages
}

func miseToolsSection(lines []string) (int, int) {
	start := -1
	for index, line := range lines {
		name, ok := miseTableHeader(strings.TrimSpace(stripMiseComment(line)))
		if !ok {
			continue
		}
		if start == -1 {
			if name == "tools" {
				start = index
			}
			continue
		}
		return start, index
	}
	if start == -1 {
		return -1, -1
	}
	return start, len(lines)
}

func parseMiseToolLine(line string) (string, string, bool) {
	semantic := strings.TrimSpace(stripMiseComment(line))
	key, value, ok := strings.Cut(semantic, "=")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return "", "", false
	}
	name := strings.Trim(key, `"'`)
	version, ok := parseTomlString(value)
	if !ok {
		return name, value, true
	}
	return name, version, true
}

func formatMiseToolLine(name string, version string) string {
	return fmt.Sprintf("%s = %q\n", formatTomlKey(name), version)
}

func commentMiseToolLine(line string, tool upgradeMiseTool, version string) string {
	reason := tool.Reason
	if reason == "" {
		reason = "this tool is no longer required by GoLazy"
	}
	if version == "" {
		version = "this release"
	}
	trimmed := strings.TrimRight(line, "\r\n")
	return fmt.Sprintf("# %s # not needed by GoLazy %s; %s.\n", strings.TrimSpace(trimmed), version, reason)
}

func commentTomlLine(line string) string {
	trimmed := strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(trimmed) == "" {
		return "#\n"
	}
	return "# " + trimmed + "\n"
}

func stripMiseComment(line string) string {
	inString := rune(0)
	escaped := false
	for index, char := range line {
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' && inString == '"' {
				escaped = true
				continue
			}
			if char == inString {
				inString = 0
			}
			continue
		}
		switch char {
		case '"', '\'':
			inString = char
		case '#':
			return line[:index]
		}
	}
	return line
}

func miseTableHeader(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	if strings.HasPrefix(line, "[[") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")), true
}

func parseTomlString(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", false
	}
	return strings.Trim(value, `"`), true
}

func formatTomlKey(name string) string {
	if name == "" {
		return name
	}
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return fmt.Sprintf("%q", name)
	}
	return name
}

func slicesClone(values []string) []string {
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func splitPhysicalLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
