package nodeservice

import (
	"encoding/json"
	"io"

	"golazy.dev/lazy/services/execservice"
)

type NodePackageManager struct {
	command string
	viaMise bool
}

func DetectNodePackageManager(dir string, runner execservice.OutputRunner) NodePackageManager {
	tools, err := CurrentInstalledTools(dir, runner)
	if err != nil {
		return DirectNPM()
	}
	for _, candidate := range []struct {
		tool    string
		manager NodePackageManager
	}{
		{"pnpm", NodePackageManager{command: "pnpm", viaMise: true}},
		{"yarn", NodePackageManager{command: "yarn", viaMise: true}},
		{"bun", NodePackageManager{command: "bun", viaMise: true}},
		{"node", NodePackageManager{command: "npm", viaMise: true}},
	} {
		if tools[candidate.tool] {
			return candidate.manager
		}
	}
	return DirectNPM()
}

func DirectNPM() NodePackageManager {
	return NodePackageManager{command: "npm"}
}

func QueryRunner(runner execservice.Runner, outputRunner execservice.OutputRunner) execservice.OutputRunner {
	if outputRunner != nil {
		return outputRunner
	}
	if runner != nil {
		return func(string, []string, execservice.Options) ([]byte, error) {
			return []byte("{}"), nil
		}
	}
	return nil
}

func (m NodePackageManager) Command() string {
	return m.command
}

func (m NodePackageManager) ViaMise() bool {
	return m.viaMise
}

func (m NodePackageManager) InstallCommand(runner execservice.Runner) (string, []string, []string) {
	args := []string{"install"}
	if m.viaMise {
		return execservice.MiseExecRunnerCommand(runner, m.command, args)
	}
	return m.command, args, nil
}

func (m NodePackageManager) RunCommand(runner execservice.Runner, command string, args []string) (string, []string, []string) {
	if m.viaMise {
		return execservice.MiseExecRunnerCommand(runner, command, args)
	}
	return command, args, nil
}

func CurrentInstalledTools(dir string, runner execservice.OutputRunner) (map[string]bool, error) {
	command := "mise"
	var env []string
	if runner == nil {
		runner = execservice.ExecOutput
		command, env = execservice.ResolveMiseCommand()
	}

	output, err := runner(command, []string{"ls", "--json", "--current", "--installed", "-C", dir}, execservice.Options{
		Dir:    dir,
		Env:    env,
		Stderr: io.Discard,
	})
	if err != nil {
		return nil, err
	}
	return parseCurrentInstalledTools(output)
}

type listedTool struct {
	Installed *bool `json:"installed"`
	Active    *bool `json:"active"`
}

func parseCurrentInstalledTools(data []byte) (map[string]bool, error) {
	var tools map[string][]listedTool
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, err
	}

	current := map[string]bool{}
	for name, entries := range tools {
		for _, entry := range entries {
			if entry.Installed != nil && !*entry.Installed {
				continue
			}
			if entry.Active != nil && !*entry.Active {
				continue
			}
			current[name] = true
			break
		}
	}
	return current, nil
}
