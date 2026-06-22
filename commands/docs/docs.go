package docs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"golazy.dev/lazydoc"
)

type Command struct {
	Dir    string
	Query  string
	JSON   bool
	Stdout io.Writer
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
	index, err := lazydoc.LoadDir(dir, "local")
	if err != nil {
		return 1, err
	}
	if c.JSON {
		return c.writeJSON(index)
	}
	if strings.TrimSpace(c.Query) != "" {
		return c.writeSearch(index)
	}
	return c.writePackages(index)
}

func (c Command) writeJSON(index *lazydoc.Index) (int, error) {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return 1, err
	}
	_, err = fmt.Fprintln(c.Stdout, string(data))
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func (c Command) writeSearch(index *lazydoc.Index) (int, error) {
	results := index.Search("local", c.Query)
	if len(results) == 0 {
		_, err := fmt.Fprintf(c.Stdout, "No package docs matched %q.\n", c.Query)
		if err != nil {
			return 1, err
		}
		return 0, nil
	}
	for _, result := range results {
		if _, err := fmt.Fprintf(c.Stdout, "%-9s %-32s %s\n", result.Kind, result.Name, result.PackagePath); err != nil {
			return 1, err
		}
	}
	return 0, nil
}

func (c Command) writePackages(index *lazydoc.Index) (int, error) {
	version, ok := index.Latest()
	if !ok {
		return 0, nil
	}
	for _, pkg := range version.Packages {
		if pkg.Synopsis != "" {
			if _, err := fmt.Fprintf(c.Stdout, "%-36s %s\n", pkg.ImportPath, pkg.Synopsis); err != nil {
				return 1, err
			}
			continue
		}
		if _, err := fmt.Fprintln(c.Stdout, pkg.ImportPath); err != nil {
			return 1, err
		}
	}
	return 0, nil
}
