package routes

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/golazy/lazy/commands"
	"github.com/golazy/lazy/commands/appcmd"
)

type Command struct {
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
	Runner commands.OutputRunner
}

type Route struct {
	Method     string          `json:"method"`
	Path       string          `json:"path"`
	Name       string          `json:"name"`
	Controller string          `json:"controller"`
	Action     string          `json:"action"`
	Namespace  string          `json:"namespace"`
	Params     map[string]bool `json:"params"`
}

func (c Command) Execute() (int, error) {
	dir := c.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return 1, fmt.Errorf("get working directory: %w", err)
		}
	}

	candidate, err := appcmd.Find(dir)
	if err != nil {
		return 1, err
	}

	runner := c.Runner
	if runner == nil {
		runner = commands.ExecOutput
	}

	output, err := runner("go", []string{
		"run",
		"-tags",
		"lazydev,printroutes",
		"./" + filepath.ToSlash(candidate),
	}, commands.Options{
		Dir:    dir,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	})
	if err != nil {
		var processExit *commands.ExitError
		if errors.As(err, &processExit) {
			return processExit.Code, nil
		}
		return 1, fmt.Errorf("run application route printer: %w", err)
	}

	routes, err := parseRoutes(output)
	if err != nil {
		return 1, err
	}
	if err := writeTable(c.Stdout, routes); err != nil {
		return 1, err
	}
	return 0, nil
}

func parseRoutes(output []byte) ([]Route, error) {
	var routes []Route
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var route Route
		if err := json.Unmarshal(line, &route); err != nil {
			return nil, fmt.Errorf("parse route output line %d: %w", lineNumber, err)
		}
		if route.Params == nil {
			route.Params = map[string]bool{}
		}
		routes = append(routes, route)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read route output: %w", err)
	}
	return routes, nil
}

func writeTable(w io.Writer, routes []Route) error {
	if w == nil {
		return nil
	}

	rows := [][]string{{"Name", "Method", "Path", "Controller#Action", "Params"}}
	for _, route := range routes {
		rows = append(rows, []string{
			route.Name,
			route.Method,
			route.Path,
			routeTarget(route),
			formatParams(route.Params),
		})
	}

	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for index, value := range row {
			if len(value) > widths[index] {
				widths[index] = len(value)
			}
		}
	}

	for _, row := range rows {
		var line strings.Builder
		for index, value := range row {
			if index > 0 {
				line.WriteString("  ")
			}
			if index == len(row)-1 {
				line.WriteString(value)
				continue
			}
			if _, err := fmt.Fprintf(&line, "%-*s", widths[index], value); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, strings.TrimRight(line.String(), " ")); err != nil {
			return err
		}
	}
	return nil
}

func routeTarget(route Route) string {
	if route.Controller == "" {
		return route.Action
	}
	if route.Action == "" {
		return route.Controller
	}
	return route.Controller + "#" + route.Action
}

func formatParams(params map[string]bool) string {
	if len(params) == 0 {
		return ""
	}
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}
