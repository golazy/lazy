package main

import (
	"context"
	"fmt"

	addonscommand "golazy.dev/lazy/commands/addons"
	jscommand "golazy.dev/lazy/commands/js"
	newcommand "golazy.dev/lazy/commands/new"
	"golazy.dev/lazy/services/addonservice"
	"golazy.dev/lazytui"
	"golazy.dev/lazytui/lazycli"
)

const lazyLogo = `   __
  / /  ___ ____  __ __
 / /__/ _ '/ _ \/ // /
/____/\_,_/_//_/\_, /
              /___/`

type rootParams struct {
	CmdPath    string `lazycli:"name=cmdpath,help=Application command path"`
	ViewPath   string `lazycli:"name=viewpath,default=app/views,help=Local view path for lazydev builds"`
	PublicPath string `lazycli:"name=publicpath,default=app/public,help=Local public path for lazydev builds"`
}

type newParams struct {
	SourceDir       string `lazycli:"name=source-dir,help=Copy the template from a local directory"`
	Version         string `lazycli:"help=Sample app version to clone"`
	SkipUpdateCheck bool   `lazycli:"name=skip-update-check,help=Skip the online lazy version check"`
	Module          string `lazycli:"positional"`
}

type tailwindParams struct {
	Input  string `lazycli:"help=Tailwind input stylesheet"`
	Output string `lazycli:"help=Compiled public stylesheet"`
	Watch  bool   `lazycli:"help=Watch source files and rebuild styles"`
}

type appPathParams struct {
	CmdPath    string `lazycli:"name=cmdpath,help=Application command path"`
	ViewPath   string `lazycli:"name=viewpath,default=app/views,help=Local view path for lazydev builds"`
	PublicPath string `lazycli:"name=publicpath,default=app/public,help=Local public path for lazydev builds"`
}

type datasetParams struct {
	Dataset string `lazycli:"positional"`
}

type upgradeParams struct {
	Target       string `lazycli:"help=Target GoLazy version"`
	Force        string `lazycli:"help=Force a one-step upgrade from this version"`
	DryRun       bool   `lazycli:"name=dry-run,help=Print the plan without writing files"`
	SkipCommands bool   `lazycli:"name=skip-commands,help=Skip follow-up commands"`
	InternalStep bool   `lazycli:"name=internal-step,help=Run one internal upgrade step"`
	From         string `lazycli:"help=Internal source version"`
	To           string `lazycli:"help=Internal target version"`
}

type nativeParams struct {
	CmdPath    string `lazycli:"name=cmdpath,help=Application command path"`
	ViewPath   string `lazycli:"name=viewpath,default=app/views,help=Local view path for lazydev builds"`
	PublicPath string `lazycli:"name=publicpath,default=app/public,help=Local public path for lazydev builds"`
	Title      string `lazycli:"help=Native window title"`
	Width      int    `lazycli:"help=Native window width"`
	Height     int    `lazycli:"help=Native window height"`
}

type nativeBuildParams struct {
	CmdPath string `lazycli:"name=cmdpath,help=Application command path"`
	Out     string `lazycli:"help=Native build output directory"`
	Target  string `lazycli:"help=Native build target"`
}

type docsParams struct {
	Dir   string `lazycli:"help=Go module directory to inspect"`
	JSON  bool   `lazycli:"help=Print package docs as JSON"`
	Query string `lazycli:"positional"`
}

type assistantParams struct {
	Codex  bool `lazycli:"help=Force Codex"`
	Claude bool `lazycli:"help=Force Claude"`
}

type addonParams struct {
	DryRun      bool     `lazycli:"name=dry-run,help=Print changes without writing files"`
	Manifest    []string `lazycli:"repeatable,help=Resolve from a local lazyaddon.toml"`
	Config      []string `lazycli:"repeatable,help=Set add-on configuration key=value"`
	UnsetConfig []string `lazycli:"name=unset-config,repeatable,help=Remove an add-on configuration key"`
	Addon       string   `lazycli:"positional"`
}

type updateAddonParams struct {
	DryRun   bool     `lazycli:"name=dry-run,help=Print changes without writing files"`
	Manifest []string `lazycli:"repeatable,help=Resolve from a local lazyaddon.toml"`
	Addons   []string `lazycli:"variadic"`
}

type cliController struct{}

func CLI(s lazycli.Scope) {
	s.Root(lazycli.Command{
		Name:         "lazy",
		Help:         "Develop and operate GoLazy applications",
		Description:  "Lazy runs applications and owns GoLazy's development, generation, upgrade, and deployment workflows.",
		Logo:         lazyLogo,
		Palette:      lazytui.GoLazyPalette,
		Controller:   lazycli.Controller[cliController]{},
		Autocomplete: true,
	})

	s.Define("new", lazycli.Command{Help: "Create a GoLazy application"})
	s.Define("routes", lazycli.Command{Help: "Print application HTTP routes"})
	s.Define("dump", lazycli.Command{Help: "Dump a development dataset"})
	s.Define("load", lazycli.Command{Help: "Load a development dataset"})
	s.Define("upgrade", lazycli.Command{Help: "Upgrade a GoLazy application"})
	s.Define("docs", lazycli.Command{Help: "Inspect Go package documentation"})
	s.Define("add", lazycli.Command{Help: "Install an add-on"})
	s.Define("remove", lazycli.Command{Help: "Remove an add-on"})
	s.Define("update", lazycli.Command{Help: "Update installed add-ons"})
	s.Define("addons", lazycli.Command{Help: "List installed add-ons"})
	s.Define("js", lazycli.Command{Help: "Build application JavaScript", Method: "JS"})
	s.Define("tailwind", lazycli.Command{Help: "Build Tailwind styles"})
	s.Define("command-center", lazycli.Command{Help: "Open the lazy command center", Hidden: true})
	s.Define("bastard", lazycli.Command{Help: "Run the experimental coding assistant", Hidden: true})
	s.Group("native", lazycli.Command{Help: "Run or build a native application"}, func(s lazycli.Scope) {
		s.Define("build", lazycli.Command{Help: "Build a native application"})
	})
}

func (*cliController) Run(ctx context.Context, params rootParams) error {
	return lazycli.Exit(executeRun(args(
		value("cmdpath", params.CmdPath), value("viewpath", params.ViewPath), value("publicpath", params.PublicPath),
	), lazycli.Stdin(ctx), lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) New(ctx context.Context, params newParams) error {
	if params.Module == "" {
		return fmt.Errorf("module is required")
	}
	if params.SourceDir != "" && params.Version != "" {
		return fmt.Errorf("--source-dir and --version cannot be used together")
	}
	version := currentVersion()
	if params.Version != "" {
		version = params.Version
	}
	return (newcommand.Command{
		Version: version, CurrentVersion: currentVersion(), SourceDir: params.SourceDir,
		GoWork: Config.GoWork, SkipUpdateCheck: params.SkipUpdateCheck,
		Stdout: lazycli.Stdout(ctx), Stderr: lazycli.Stderr(ctx),
	}).Execute(params.Module)
}

func (*cliController) Routes(ctx context.Context, params appPathParams) error {
	return lazycli.Exit(executeRoutes(args(value("cmdpath", params.CmdPath), value("viewpath", params.ViewPath), value("publicpath", params.PublicPath)), lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) Dump(ctx context.Context, params datasetParams) error {
	return lazycli.Exit(executeDatasetDump([]string{params.Dataset}, lazycli.Stdin(ctx), lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) Load(ctx context.Context, params datasetParams) error {
	return lazycli.Exit(executeDatasetLoad([]string{params.Dataset}, lazycli.Stdin(ctx), lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) Upgrade(ctx context.Context, params upgradeParams) error {
	return lazycli.Exit(executeUpgrade(args(
		value("target", params.Target), value("force", params.Force), boolean("dry-run", params.DryRun),
		boolean("skip-commands", params.SkipCommands), boolean("internal-step", params.InternalStep),
		value("from", params.From), value("to", params.To),
	), lazycli.Stdin(ctx), lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) Native(ctx context.Context, params nativeParams) error {
	return lazycli.Exit(executeNative(args(
		value("cmdpath", params.CmdPath), value("viewpath", params.ViewPath), value("publicpath", params.PublicPath),
		value("title", params.Title), integer("width", params.Width), integer("height", params.Height),
	), lazycli.Stdin(ctx), lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) Build(ctx context.Context, params nativeBuildParams) error {
	return lazycli.Exit(executeNativeBuild(args(value("cmdpath", params.CmdPath), value("out", params.Out), value("target", params.Target)), lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) Docs(ctx context.Context, params docsParams) error {
	values := args(value("dir", params.Dir), boolean("json", params.JSON))
	if params.Query != "" {
		values = append(values, params.Query)
	}
	return lazycli.Exit(executeDocs(values, lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) CommandCenter(ctx context.Context, _ struct{}) error {
	return lazycli.Exit(executeCommandCenter(nil, lazycli.Stdin(ctx), lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) Bastard(ctx context.Context, params assistantParams) error {
	return lazycli.Exit(executeBastard(args(boolean("codex", params.Codex), boolean("claude", params.Claude)), lazycli.Stdin(ctx), lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) JS(ctx context.Context, _ struct{}) error {
	code, err := (jscommand.Command{Stdout: lazycli.Stdout(ctx), Stderr: lazycli.Stderr(ctx)}).Execute()
	if err != nil {
		return err
	}
	return lazycli.Exit(code)
}

func (*cliController) Tailwind(ctx context.Context, params tailwindParams) error {
	return lazycli.Exit(executeTailwind(args(value("input", params.Input), value("output", params.Output), boolean("watch", params.Watch)), lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) Add(ctx context.Context, params addonParams) error {
	return runAddon(ctx, "add", params, nil)
}

func (*cliController) Remove(ctx context.Context, params addonParams) error {
	return runAddon(ctx, "remove", params, nil)
}

func (*cliController) Update(ctx context.Context, params updateAddonParams) error {
	values := args(boolean("dry-run", params.DryRun))
	for _, manifest := range params.Manifest {
		values = append(values, "--manifest", manifest)
	}
	values = append(values, params.Addons...)
	return lazycli.Exit(runAddonCommand("update", values, lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func (*cliController) Addons(ctx context.Context, _ struct{}) error {
	return lazycli.Exit(runAddonCommand("addons", nil, lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func runAddon(ctx context.Context, name string, params addonParams, extra []string) error {
	values := args(boolean("dry-run", params.DryRun))
	for _, manifest := range params.Manifest {
		values = append(values, "--manifest", manifest)
	}
	for _, configured := range params.Config {
		values = append(values, "--config", configured)
	}
	for _, unset := range params.UnsetConfig {
		values = append(values, "--unset-config", unset)
	}
	values = append(values, extra...)
	if params.Addon != "" {
		values = append(values, params.Addon)
	}
	return lazycli.Exit(runAddonCommand(name, values, lazycli.Stdout(ctx), lazycli.Stderr(ctx)))
}

func args(groups ...[]string) []string {
	var result []string
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
}

func value(name, configured string) []string {
	if configured == "" {
		return nil
	}
	return []string{"--" + name, configured}
}

func boolean(name string, configured bool) []string {
	if !configured {
		return nil
	}
	return []string{"--" + name}
}

func integer(name string, configured int) []string {
	if configured == 0 {
		return nil
	}
	return []string{"--" + name, fmt.Sprint(configured)}
}

func runAddonCommand(name string, values []string, stdout, stderr interface{ Write([]byte) (int, error) }) int {
	code, err := (addonscommand.Command{
		FrameworkVersion: currentVersion(), Stdout: stdout, Stderr: stderr,
		Sources: []addonservice.Source{addonservice.FirstPartySource(currentVersion())},
	}).Execute(name, values)
	if err != nil {
		fmt.Fprintf(stderr, "lazy: %v\n", err)
	}
	return code
}
