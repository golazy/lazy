// Package addons implements lazy add, remove, update, and addons.
package addons

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"golazy.dev/lazy/services/addonservice"
)

type Command struct {
	Dir              string
	FrameworkVersion string
	Sources          []addonservice.Source
	Stdout           io.Writer
	Stderr           io.Writer
}

func (command Command) Execute(name string, args []string) (int, error) {
	switch name {
	case "add":
		return command.executeAdd(args)
	case "remove":
		return command.executeRemove(args)
	case "update":
		return command.executeUpdate(args)
	case "addons":
		return command.executeList(args)
	default:
		return 1, fmt.Errorf("unknown add-on command %q", name)
	}
}

func (command Command) executeAdd(args []string) (int, error) {
	flags, dryRun, manifests := command.flags("add")
	configs := new(configList)
	flags.Var(configs, "config", "set non-secret add-on configuration as key=value (repeatable)")
	unsetConfigs := new(configKeyList)
	flags.Var(unsetConfigs, "unset-config", "remove a non-secret add-on configuration key (repeatable)")
	if err := flags.Parse(args); err != nil {
		return 1, nil
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(command.err(), "lazy: usage: lazy add [--dry-run] [--manifest <path>] [--config key=value] [--unset-config key] <addon>[@version]")
		return 1, nil
	}
	plan, err := command.manager(manifests).AddWithOptions(flags.Arg(0), addonservice.AddOptions{
		Config:      configs.Map(),
		UnsetConfig: unsetConfigs.Values(),
	})
	if err != nil {
		return 1, err
	}
	return command.finish(plan, *dryRun)
}

func (command Command) executeRemove(args []string) (int, error) {
	flags, dryRun, manifests := command.flags("remove")
	if err := flags.Parse(args); err != nil {
		return 1, nil
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(command.err(), "lazy: usage: lazy remove [--dry-run] [--manifest <path>] <addon>")
		return 1, nil
	}
	plan, err := command.manager(manifests).Remove(flags.Arg(0))
	if err != nil {
		return 1, err
	}
	return command.finish(plan, *dryRun)
}

func (command Command) executeUpdate(args []string) (int, error) {
	flags, dryRun, manifests := command.flags("update")
	if err := flags.Parse(args); err != nil {
		return 1, nil
	}
	plan, err := command.manager(manifests).Update(flags.Args()...)
	if err != nil {
		return 1, err
	}
	return command.finish(plan, *dryRun)
}

func (command Command) executeList(args []string) (int, error) {
	flags := flag.NewFlagSet("addons", flag.ContinueOnError)
	flags.SetOutput(command.err())
	if err := flags.Parse(args); err != nil {
		return 1, nil
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(command.err(), "lazy: usage: lazy addons")
		return 1, nil
	}
	state, err := addonservice.ReadState(command.Dir)
	if err != nil {
		return 1, err
	}
	if len(state.Addons) == 0 {
		fmt.Fprintln(command.out(), "No add-ons installed.")
		return 0, nil
	}
	fmt.Fprintln(command.out(), "ADD-ON\tVERSION\tSELECTION\tPACKAGE")
	for _, addon := range state.Addons {
		selection := "required"
		if addon.Direct {
			selection = "direct"
		}
		fmt.Fprintf(command.out(), "%s\t%s\t%s\t%s@%s\n", addon.ID, addon.Version, selection, addon.Package, addon.PackageVersion)
	}
	return 0, nil
}

func (command Command) flags(name string) (*flag.FlagSet, *bool, *manifestList) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(command.err())
	dryRun := flags.Bool("dry-run", false, "print the add-on changes without writing files")
	manifests := new(manifestList)
	flags.Var(manifests, "manifest", "resolve add-ons from a local lazyaddon.toml (repeatable)")
	return flags, dryRun, manifests
}

func (command Command) manager(manifests *manifestList) addonservice.Manager {
	return addonservice.Manager{
		Dir:              command.Dir,
		FrameworkVersion: command.FrameworkVersion,
		ManifestPaths:    append([]string(nil), (*manifests)...),
		Sources:          append([]addonservice.Source(nil), command.Sources...),
	}
}

func (command Command) finish(plan addonservice.Plan, dryRun bool) (int, error) {
	if dryRun {
		plan.Print(command.out())
		return 0, nil
	}
	if !plan.Changed() {
		fmt.Fprintln(command.out(), "  no changes")
		return 0, nil
	}
	if err := plan.Apply(); err != nil {
		return 1, err
	}
	for _, edit := range plan.Result.Files {
		verb := string(edit.Kind)
		switch verb {
		case "create":
			verb = "created"
		case "update":
			verb = "updated"
		case "delete":
			verb = "removed"
		}
		fmt.Fprintf(command.out(), "  %s %s\n", verb, edit.Path)
	}
	for _, edit := range plan.ModeChanges {
		fmt.Fprintf(command.out(), "  updated mode %o %s\n", edit.After, edit.Path)
	}
	return 0, nil
}

func (command Command) out() io.Writer {
	if command.Stdout == nil {
		return io.Discard
	}
	return command.Stdout
}

func (command Command) err() io.Writer {
	if command.Stderr == nil {
		return io.Discard
	}
	return command.Stderr
}

type manifestList []string

func (list *manifestList) String() string { return strings.Join(*list, ",") }

func (list *manifestList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("manifest path is required")
	}
	*list = append(*list, value)
	return nil
}

type configList map[string]string

func (list *configList) String() string {
	if list == nil || len(*list) == 0 {
		return ""
	}
	keys := make([]string, 0, len(*list))
	for key := range *list {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, key+"="+(*list)[key])
	}
	return strings.Join(values, ",")
}

func (list *configList) Set(value string) error {
	key, configured, found := strings.Cut(value, "=")
	if !found || strings.TrimSpace(key) != key || key == "" {
		return fmt.Errorf("add-on configuration must use key=value")
	}
	if *list == nil {
		*list = configList{}
	}
	if _, exists := (*list)[key]; exists {
		return fmt.Errorf("add-on configuration key %q is repeated", key)
	}
	(*list)[key] = configured
	return nil
}

func (list *configList) Map() map[string]string {
	if list == nil || len(*list) == 0 {
		return nil
	}
	config := make(map[string]string, len(*list))
	for key, value := range *list {
		config[key] = value
	}
	return config
}

type configKeyList []string

func (list *configKeyList) String() string {
	if list == nil {
		return ""
	}
	return strings.Join(*list, ",")
}

func (list *configKeyList) Set(value string) error {
	if value == "" || strings.TrimSpace(value) != value || strings.Contains(value, "=") {
		return fmt.Errorf("add-on configuration key is required")
	}
	for _, previous := range *list {
		if previous == value {
			return fmt.Errorf("add-on configuration key %q is repeated", value)
		}
	}
	*list = append(*list, value)
	return nil
}

func (list *configKeyList) Values() []string {
	if list == nil {
		return nil
	}
	return append([]string(nil), (*list)...)
}
