package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/golazy/lazy/commands/appcmd"
	bastardcommand "github.com/golazy/lazy/commands/bastard"
	commandcenter "github.com/golazy/lazy/commands/commandcenter"
	docscommand "github.com/golazy/lazy/commands/docs"
	jscommand "github.com/golazy/lazy/commands/js"
	"github.com/golazy/lazy/commands/lazyconfig"
	"github.com/golazy/lazy/commands/lazytmux"
	newcommand "github.com/golazy/lazy/commands/new"
	routescommand "github.com/golazy/lazy/commands/routes"
	runcommand "github.com/golazy/lazy/commands/run"
	tailwindcommand "github.com/golazy/lazy/commands/tailwind"
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func execute(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	args, skipVersionCheck := removeSkipVersionCheckFlag(args)
	if !skipVersionCheck {
		if handled, code := maybeExecuteProjectVersion(args, stdin, stdout, stderr); handled {
			return code
		}
	}

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
	case "tailwind":
		return executeTailwind(args[1:], stdout, stderr)
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
	case "docs":
		return executeDocs(args[1:], stdout, stderr)
	case "command-center":
		return executeCommandCenter(args[1:], stdin, stdout, stderr)
	case "bastard":
		return executeBastard(args[1:], stdin, stdout, stderr)
	default:
		if strings.HasPrefix(args[0], "-") {
			return executeRun(args, stdin, stdout, stderr)
		}
		fmt.Fprintf(stderr, "lazy: unknown command %q\n", args[0])
		return 1
	}
}

func executeBastard(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	code, err := (bastardcommand.Command{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Help: func() {
			printUsage(stdout)
		},
	}).Execute(args)
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
	}
	return code
}

func printUsage(stdout io.Writer) {
	fmt.Fprintln(stdout, "usage: lazy [--skip-version-check] [--cmdpath <path>] [--viewpath <path>]")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "commands:")
	fmt.Fprintln(stdout, "  lazy")
	fmt.Fprintln(stdout, "  lazy new <module>")
	fmt.Fprintln(stdout, "  lazy routes")
	fmt.Fprintln(stdout, "  lazy docs")
	fmt.Fprintln(stdout, "  lazy js")
	fmt.Fprintln(stdout, "  lazy tailwind")
	fmt.Fprintln(stdout, "  lazy --version")
}

func removeSkipVersionCheckFlag(args []string) ([]string, bool) {
	for index, arg := range args {
		if arg != skipVersionCheckFlag {
			continue
		}
		filtered := make([]string, 0, len(args)-1)
		filtered = append(filtered, args[:index]...)
		for _, laterArg := range args[index+1:] {
			if laterArg != skipVersionCheckFlag {
				filtered = append(filtered, laterArg)
			}
		}
		return filtered, true
	}
	return args, false
}

func executeDocs(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("docs", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", "", "Go module directory to inspect")
	jsonOutput := flags.Bool("json", false, "print package docs as JSON")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderr, "lazy: usage: lazy docs [--dir <path>] [--json] [query]")
		return 1
	}
	query := ""
	if flags.NArg() == 1 {
		query = flags.Arg(0)
	}

	code, err := (docscommand.Command{
		Dir:    *dir,
		Query:  query,
		JSON:   *jsonOutput,
		Stdout: stdout,
	}).Execute()
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
	}
	return code
}

func executeCommandCenter(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "lazy: command-center does not accept arguments")
		return 1
	}
	code, err := (commandcenter.Command{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}).Execute()
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
	}
	return code
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
		fmt.Fprintln(stderr, "lazy: usage: lazy [--skip-version-check] [--cmdpath <path>] [--viewpath <path>]")
		return 1
	}

	if os.Getenv(lazytmux.InSessionEnv) != "1" {
		config, ok, err := lazyconfig.LoadIfExists(".")
		if err != nil {
			fmt.Fprintf(stderr, "lazy: %v\n", err)
			return 1
		}
		if ok {
			code, err := (lazytmux.Command{
				Dir:      ".",
				CmdPath:  *cmdPath,
				ViewPath: *viewPath,
				Config:   config,
				Stdin:    stdin,
				Stdout:   stdout,
				Stderr:   stderr,
			}).Execute()
			if err != nil {
				fmt.Fprintf(stderr, "lazy: %v\n", err)
			}
			return code
		}
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

func executeTailwind(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("tailwind", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("input", "", "Tailwind input stylesheet")
	output := flags.String("output", "", "compiled public stylesheet")
	watch := flags.Bool("watch", false, "watch source files and rebuild styles")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "lazy: usage: lazy tailwind [--input <path>] [--output <path>] [--watch]")
		return 1
	}

	code, err := (tailwindcommand.Command{
		Input:  *input,
		Output: *output,
		Watch:  *watch,
		Stdout: stdout,
		Stderr: stderr,
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
