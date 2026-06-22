package miseconfig

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type GoToolCheck struct {
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	DryRun bool
}

func (c GoToolCheck) Execute() error {
	dir := c.Dir
	if strings.TrimSpace(dir) == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}
	path := filepath.Join(dir, "mise.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read mise.toml: %w", err)
	}
	updated, found := RemoveGoTool(data)
	if !found {
		return nil
	}

	stdout := c.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := c.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	if c.DryRun {
		fmt.Fprintln(stdout, "  would remove Go from mise.toml")
		return nil
	}

	fmt.Fprintln(stderr, "lazy: mise.toml lists Go under [tools].")
	fmt.Fprintln(stderr, "Go already bundles multi-version support through the go directive and toolchain selection, so GoLazy development should not ask mise to install Go.")
	fmt.Fprint(stderr, "Remove go from mise.toml? [y/N] ")
	if !confirmed(c.Stdin) {
		fmt.Fprintln(stderr)
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect mise.toml: %w", err)
	}
	if err := os.WriteFile(path, updated, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write mise.toml: %w", err)
	}
	fmt.Fprintln(stdout, "  removed Go from mise.toml")
	return nil
}

func RemoveGoTool(data []byte) ([]byte, bool) {
	lines := splitLines(data)
	inTools := false
	removed := false
	var out strings.Builder
	for _, line := range lines {
		semantic := strings.TrimSpace(stripComment(line))
		if tableName, ok := tableHeader(semantic); ok {
			inTools = tableName == "tools"
		}
		if inTools && isGoToolLine(semantic) {
			removed = true
			continue
		}
		out.WriteString(line)
	}
	if !removed {
		return data, false
	}
	return []byte(out.String()), true
}

func confirmed(stdin io.Reader) bool {
	if stdin == nil {
		return false
	}
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}

func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	value := string(data)
	var lines []string
	for len(value) > 0 {
		index := strings.IndexByte(value, '\n')
		if index == -1 {
			lines = append(lines, value)
			break
		}
		lines = append(lines, value[:index+1])
		value = value[index+1:]
	}
	return lines
}

func stripComment(line string) string {
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

func tableHeader(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	if strings.HasPrefix(line, "[[") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")), true
}

func isGoToolLine(line string) bool {
	key, _, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	return strings.Trim(strings.TrimSpace(key), `"'`) == "go"
}
