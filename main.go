package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/golazy/lazy/commands/appcmd"
	jscommand "github.com/golazy/lazy/commands/js"
	newcommand "github.com/golazy/lazy/commands/new"
	routescommand "github.com/golazy/lazy/commands/routes"
	runcommand "github.com/golazy/lazy/commands/run"
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func execute(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		return executeRun(args, stdin, stdout, stderr)
	}

	switch args[0] {
	case "--version":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "lazy: version does not accept arguments")
			return 1
		}
		fmt.Fprintf(stdout, "lazy %s\n", currentVersion())
		return 0
	case "js":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "lazy: js does not accept arguments")
			return 1
		}
		code, err := (jscommand.Command{
			Stdout: stdout,
			Stderr: stderr,
		}).Execute()
		if err != nil {
			fmt.Fprintf(stderr, "lazy: %v\n", err)
		}
		return code
	case "new":
		flags := flag.NewFlagSet("new", flag.ContinueOnError)
		flags.SetOutput(stderr)
		sourceDir := flags.String("source-dir", "", "copy the template from a local directory")
		if err := flags.Parse(args[1:]); err != nil {
			return 1
		}
		if flags.NArg() != 1 {
			fmt.Fprintln(stderr, "lazy: usage: lazy new <module>")
			return 1
		}
		err := (newcommand.Command{
			Version:   currentVersion(),
			SourceDir: *sourceDir,
			Stdout:    stdout,
		}).Execute(flags.Arg(0))
		if err != nil {
			fmt.Fprintf(stderr, "lazy: %v\n", err)
			return 1
		}
		return 0
	case "routes":
		return executeRoutes(args[1:], stdout, stderr)
	default:
		if strings.HasPrefix(args[0], "-") {
			return executeRun(args, stdin, stdout, stderr)
		}
		fmt.Fprintf(stderr, "lazy: unknown command %q\n", args[0])
		return 1
	}
}

func executeRun(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("lazy", flag.ContinueOnError)
	flags.SetOutput(stderr)
	cmdPath := flags.String("cmdpath", "", "application command path")
	viewPath := flags.String("viewpath", appcmd.DefaultViewPath, "local view path for lazydev builds")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "lazy: usage: lazy [--cmdpath <path>] [--viewpath <path>]")
		return 1
	}

	code, err := (runcommand.Command{
		CmdPath:  *cmdPath,
		ViewPath: *viewPath,
		Stdin:    stdin,
		Stdout:   stdout,
		Stderr:   stderr,
	}).Execute()
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
	}
	return code
}

func executeRoutes(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("routes", flag.ContinueOnError)
	flags.SetOutput(stderr)
	cmdPath := flags.String("cmdpath", "", "application command path")
	viewPath := flags.String("viewpath", appcmd.DefaultViewPath, "local view path for lazydev builds")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "lazy: usage: lazy routes [--cmdpath <path>] [--viewpath <path>]")
		return 1
	}

	code, err := (routescommand.Command{
		CmdPath:  *cmdPath,
		ViewPath: *viewPath,
		Stdout:   stdout,
		Stderr:   stderr,
	}).Execute()
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
	}
	return code
}
