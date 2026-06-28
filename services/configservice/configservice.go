package configservice

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const Filename = "lazy.toml"

type Config struct {
	Tmux     Tmux
	Services []Service
	Runners  []Process
	Programs []Program
}

type Tmux struct {
	Session string
}

type Service struct {
	Name string
}

type Process struct {
	Name    string
	Command string
}

type Program struct {
	Name    string
	Command string
	Window  string
}

func Load(root string) (Config, error) {
	path := filepath.Join(root, Filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", Filename, err)
	}
	config, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", Filename, err)
	}
	return config, nil
}

func LoadIfExists(root string) (Config, bool, error) {
	path := filepath.Join(root, Filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, nil
		}
		return Config{}, false, fmt.Errorf("read %s: %w", Filename, err)
	}
	config, err := Parse(data)
	if err != nil {
		return Config{}, false, fmt.Errorf("parse %s: %w", Filename, err)
	}
	return config, true, nil
}

func Parse(data []byte) (Config, error) {
	var config Config
	section := ""
	activeIndex := -1

	lines := strings.Split(string(data), "\n")
	for index, rawLine := range lines {
		lineNumber := index + 1
		line := strings.TrimSpace(stripComment(rawLine))
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[[") {
			if !strings.HasSuffix(line, "]]") {
				return Config{}, fmt.Errorf("line %d: malformed array table header", lineNumber)
			}
			section = strings.TrimSpace(line[2 : len(line)-2])
			activeIndex = -1
			switch section {
			case "services", "service":
				config.Services = append(config.Services, Service{})
				activeIndex = len(config.Services) - 1
			case "runners", "runner":
				config.Runners = append(config.Runners, Process{})
				activeIndex = len(config.Runners) - 1
			case "programs", "program":
				config.Programs = append(config.Programs, Program{})
				activeIndex = len(config.Programs) - 1
			default:
				return Config{}, fmt.Errorf("line %d: unknown array table %q", lineNumber, section)
			}
			continue
		}

		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return Config{}, fmt.Errorf("line %d: malformed section header", lineNumber)
			}
			section = strings.TrimSpace(line[1 : len(line)-1])
			activeIndex = -1
			if section != "tmux" {
				return Config{}, fmt.Errorf("line %d: unknown section %q", lineNumber, section)
			}
			continue
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("line %d: expected key = value", lineNumber)
		}
		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)

		if err := assignValue(&config, section, activeIndex, key, rawValue); err != nil {
			return Config{}, fmt.Errorf("line %d: %w", lineNumber, err)
		}
	}

	config = normalize(config)
	if err := validate(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func assignValue(config *Config, section string, activeIndex int, key string, rawValue string) error {
	if section == "" {
		switch key {
		case "services":
			names, err := parseStringArray(rawValue)
			if err != nil {
				return err
			}
			for _, name := range names {
				config.Services = append(config.Services, Service{Name: name})
			}
			return nil
		default:
			return fmt.Errorf("unknown top-level key %q", key)
		}
	}

	switch section {
	case "tmux":
		if key != "session" {
			return fmt.Errorf("unknown tmux key %q", key)
		}
		value, err := parseString(rawValue)
		if err != nil {
			return err
		}
		config.Tmux.Session = value
		return nil
	case "services", "service":
		if activeIndex < 0 || activeIndex >= len(config.Services) {
			return fmt.Errorf("%q values must appear inside [[services]]", key)
		}
		if key != "name" {
			return fmt.Errorf("unknown service key %q", key)
		}
		value, err := parseString(rawValue)
		if err != nil {
			return err
		}
		config.Services[activeIndex].Name = value
		return nil
	case "runners", "runner":
		if activeIndex < 0 || activeIndex >= len(config.Runners) {
			return fmt.Errorf("%q values must appear inside [[runners]]", key)
		}
		value, err := parseString(rawValue)
		if err != nil {
			return err
		}
		switch key {
		case "name":
			config.Runners[activeIndex].Name = value
		case "command":
			config.Runners[activeIndex].Command = value
		default:
			return fmt.Errorf("unknown runner key %q", key)
		}
		return nil
	case "programs", "program":
		if activeIndex < 0 || activeIndex >= len(config.Programs) {
			return fmt.Errorf("%q values must appear inside [[programs]]", key)
		}
		value, err := parseString(rawValue)
		if err != nil {
			return err
		}
		switch key {
		case "name":
			config.Programs[activeIndex].Name = value
		case "command":
			config.Programs[activeIndex].Command = value
		case "window":
			config.Programs[activeIndex].Window = value
		default:
			return fmt.Errorf("unknown program key %q", key)
		}
		return nil
	default:
		return fmt.Errorf("unknown section %q", section)
	}
}

func normalize(config Config) Config {
	for index := range config.Services {
		config.Services[index].Name = strings.TrimSpace(config.Services[index].Name)
	}
	for index := range config.Runners {
		config.Runners[index].Name = strings.TrimSpace(config.Runners[index].Name)
		config.Runners[index].Command = strings.TrimSpace(config.Runners[index].Command)
	}
	for index := range config.Programs {
		config.Programs[index].Name = strings.TrimSpace(config.Programs[index].Name)
		config.Programs[index].Command = strings.TrimSpace(config.Programs[index].Command)
		config.Programs[index].Window = strings.TrimSpace(config.Programs[index].Window)
	}
	config.Tmux.Session = strings.TrimSpace(config.Tmux.Session)
	return config
}

func validate(config Config) error {
	seenServices := map[string]bool{}
	for _, service := range config.Services {
		if service.Name == "" {
			return fmt.Errorf("service name is required")
		}
		if seenServices[service.Name] {
			return fmt.Errorf("service %q is already declared", service.Name)
		}
		seenServices[service.Name] = true
	}

	if err := validateProcesses("runner", config.Runners); err != nil {
		return err
	}

	seenPrograms := map[string]bool{}
	for _, program := range config.Programs {
		if program.Name == "" {
			return fmt.Errorf("program name is required")
		}
		if program.Command == "" {
			return fmt.Errorf("program %q is missing command", program.Name)
		}
		if seenPrograms[program.Name] {
			return fmt.Errorf("program %q is already declared", program.Name)
		}
		seenPrograms[program.Name] = true
	}
	return nil
}

func validateProcesses(kind string, processes []Process) error {
	seen := map[string]bool{}
	for _, process := range processes {
		if process.Name == "" {
			return fmt.Errorf("%s name is required", kind)
		}
		if process.Command == "" {
			return fmt.Errorf("%s %q is missing command", kind, process.Name)
		}
		if seen[process.Name] {
			return fmt.Errorf("%s %q is already declared", kind, process.Name)
		}
		seen[process.Name] = true
	}
	return nil
}

func ServiceNames(config Config) []string {
	names := make([]string, 0, len(config.Services))
	for _, service := range config.Services {
		names = append(names, service.Name)
	}
	sort.Strings(names)
	return names
}

func stripComment(line string) string {
	inString := false
	escaped := false
	for index, char := range line {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && inString {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
			continue
		}
		if char == '#' && !inString {
			return line[:index]
		}
	}
	return line
}

func parseString(raw string) (string, error) {
	value, err := strconv.Unquote(raw)
	if err != nil {
		return "", fmt.Errorf("expected quoted string")
	}
	return value, nil
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

	var values []string
	for raw != "" {
		raw = strings.TrimSpace(raw)
		if !strings.HasPrefix(raw, "\"") {
			return nil, fmt.Errorf("expected quoted string in array")
		}
		value, rest, err := parseLeadingString(raw)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
		rest = strings.TrimSpace(rest)
		if rest == "" {
			break
		}
		if !strings.HasPrefix(rest, ",") {
			return nil, fmt.Errorf("expected comma in array")
		}
		raw = strings.TrimSpace(rest[1:])
	}
	return values, nil
}

func parseLeadingString(raw string) (string, string, error) {
	escaped := false
	for index := 1; index < len(raw); index++ {
		char := raw[index]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == '"' {
			value, err := strconv.Unquote(raw[:index+1])
			if err != nil {
				return "", "", fmt.Errorf("expected quoted string")
			}
			return value, raw[index+1:], nil
		}
	}
	return "", "", fmt.Errorf("unterminated string")
}
