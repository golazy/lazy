package main

import (
	"fmt"
	"io"
	"os"

	newcommand "github.com/golazy/lazy/commands/new"
	runcommand "github.com/golazy/lazy/commands/run"
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func execute(
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) int {
	if len(args) == 0 {
		code, err := (runcommand.Command{
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
		}).Execute()
		if err != nil {
			fmt.Fprintf(stderr, "lazy: %v\n", err)
		}
		return code
	}

	switch args[0] {
	case "--version":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "lazy: version does not accept arguments")
			return 1
		}
		fmt.Fprintf(stdout, "lazy %s\n", currentVersion())
		return 0
	case "new":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "lazy: usage: lazy new <module>")
			return 1
		}
		err := (newcommand.Command{
			Version: currentVersion(),
			Stdout:  stdout,
		}).Execute(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "lazy: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "lazy: unknown command %q\n", args[0])
		return 1
	}
}
