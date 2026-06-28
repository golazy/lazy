package commandcenterservice

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golazy.dev/lazy/services/configservice"
	"golazy.dev/lazy/services/execservice"
)

type Command struct {
	Dir     string
	Session string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Runner  execservice.Runner
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

	config, _, err := configservice.LoadIfExists(dir)
	if err != nil {
		return 1, err
	}

	stdout := c.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	fmt.Fprintln(stdout, "lazy command center")
	if session := strings.TrimSpace(c.Session); session != "" {
		fmt.Fprintf(stdout, "session: %s\n", session)
	}
	printSummary(stdout, config)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "commands: services, quit")

	stdin := c.Stdin
	if stdin == nil {
		return 0, nil
	}

	scanner := bufio.NewScanner(stdin)
	for {
		fmt.Fprint(stdout, "lazy> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		switch input {
		case "", "help":
			fmt.Fprintln(stdout, "commands: services, quit")
		case "services":
			printServices(stdout, config)
		case "quit", "q", "exit":
			return 0, nil
		default:
			fmt.Fprintf(stdout, "unknown command: %s\n", input)
		}
	}
	if err := scanner.Err(); err != nil {
		return 1, fmt.Errorf("read command center input: %w", err)
	}
	return 0, nil
}

func printSummary(stdout io.Writer, config configservice.Config) {
	fmt.Fprintf(stdout, "services: %d\n", len(config.Services))
	fmt.Fprintf(stdout, "runners: %d\n", len(config.Runners))
	fmt.Fprintf(stdout, "programs: %d\n", len(config.Programs))
}

func printServices(stdout io.Writer, config configservice.Config) {
	if len(config.Services) == 0 {
		fmt.Fprintln(stdout, "no services configured")
		return
	}
	for _, service := range config.Services {
		fmt.Fprintf(stdout, "%s: start=mise run %s:start\n", service.Name, service.Name)
	}
}
