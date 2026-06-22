package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golazy.dev/lazy/commands/appcmd"
	bastardcommand "golazy.dev/lazy/commands/bastard"
	commandcenter "golazy.dev/lazy/commands/commandcenter"
	docscommand "golazy.dev/lazy/commands/docs"
	jscommand "golazy.dev/lazy/commands/js"
	"golazy.dev/lazy/commands/lazyconfig"
	"golazy.dev/lazy/commands/lazytmux"
	"golazy.dev/lazy/commands/miseconfig"
	nativecommand "golazy.dev/lazy/commands/native"
	newcommand "golazy.dev/lazy/commands/new"
	routescommand "golazy.dev/lazy/commands/routes"
	runcommand "golazy.dev/lazy/commands/run"
	tailwindcommand "golazy.dev/lazy/commands/tailwind"
	upgradecommand "golazy.dev/lazy/commands/upgrade"
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
			Stderr:    stderr,
		}).Execute(flags.Arg(0))
		if err != nil {
			fmt.Fprintf(stderr, "lazy: %v\n", err)
			return 1
		}
		return 0
	case "routes":
		return executeRoutes(args[1:], stdout, stderr)
	case "upgrade":
		return executeUpgrade(args[1:], stdin, stdout, stderr)
	case "native":
		return executeNative(args[1:], stdin, stdout, stderr)
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
	fmt.Fprintln(stdout, "  lazy upgrade")
	fmt.Fprintln(stdout, "  lazy native")
	fmt.Fprintln(stdout, "  lazy native build")
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

func executeUpgrade(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	flags.SetOutput(stderr)
	target := flags.String("target", "", "target GoLazy version")
	force := flags.String("force", "", "force a one-step upgrade from this GoLazy version")
	dryRun := flags.Bool("dry-run", false, "print the upgrade plan without writing files")
	skipCommands := flags.Bool("skip-commands", false, "skip follow-up commands such as go test and go vet")
	internalStep := flags.Bool("internal-step", false, "run one internal upgrade step")
	from := flags.String("from", "", "source GoLazy version for internal upgrade steps")
	to := flags.String("to", "", "target GoLazy version for internal upgrade steps")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "lazy: usage: lazy upgrade [--target <version>] [--dry-run] [--skip-commands]")
		return 1
	}

	code, err := (upgradecommand.Command{
		Target:       *target,
		Force:        *force,
		From:         *from,
		To:           *to,
		InternalStep: *internalStep,
		DryRun:       *dryRun,
		SkipCommands: *skipCommands,
		Stdin:        stdin,
		Stdout:       stdout,
		Stderr:       stderr,
	}).Execute()
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
	}
	return code
}

func executeNative(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "build" {
		return executeNativeBuild(args[1:], stdout, stderr)
	}

	flags := flag.NewFlagSet("native", flag.ContinueOnError)
	flags.SetOutput(stderr)
	cmdPath := flags.String("cmdpath", "", "application command path")
	viewPath := flags.String("viewpath", appcmd.DefaultViewPath, "local view path for lazydev builds")
	title := flags.String("title", "", "native window title")
	width := flags.Int("width", 0, "native window width")
	height := flags.Int("height", 0, "native window height")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "lazy: usage: lazy native [--cmdpath <path>] [--viewpath <path>] [--title <title>] [--width <px>] [--height <px>]")
		return 1
	}

	code, err := (nativecommand.Command{
		CmdPath:  *cmdPath,
		ViewPath: *viewPath,
		Title:    *title,
		Width:    *width,
		Height:   *height,
		Stdin:    stdin,
		Stdout:   stdout,
		Stderr:   stderr,
	}).ExecuteDev()
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
	}
	return code
}

func executeNativeBuild(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("native build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	cmdPath := flags.String("cmdpath", "", "application command path")
	out := flags.String("out", "", "native build output directory")
	target := flags.String("target", "", "native build target; only the current platform is supported")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "lazy: usage: lazy native build [--target <current>] [--cmdpath <path>] [--out <dir>]")
		return 1
	}

	code, err := (nativecommand.Command{
		CmdPath: *cmdPath,
		Out:     *out,
		Target:  *target,
		Stdout:  stdout,
		Stderr:  stderr,
	}).ExecuteBuild()
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
	}
	return code
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

	if err := (miseconfig.GoToolCheck{
		Dir:    ".",
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}).Execute(); err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
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
