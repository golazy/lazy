package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func ResolveMiseCommand() (string, []string) {
	if _, err := exec.LookPath("mise"); err == nil {
		return "mise", nil
	}

	for _, candidate := range miseCommandCandidates() {
		if !isExecutableFile(candidate) {
			continue
		}
		dir := filepath.Dir(candidate)
		return candidate, []string{"PATH=" + prependPathDir(os.Getenv("PATH"), dir)}
	}

	return "mise", nil
}

func MiseExecCommand(command string, args []string) (string, []string, []string) {
	miseCommand, miseEnv := ResolveMiseCommand()
	return miseCommand, append([]string{"exec", "--", command}, args...), miseEnv
}

func MiseExecRunnerCommand(runner Runner, command string, args []string) (string, []string, []string) {
	if runner == nil {
		return MiseExecCommand(command, args)
	}
	return "mise", append([]string{"exec", "--", command}, args...), nil
}

func MiseExecOutputRunnerCommand(runner OutputRunner, command string, args []string) (string, []string, []string) {
	if runner == nil {
		return MiseExecCommand(command, args)
	}
	return "mise", append([]string{"exec", "--", command}, args...), nil
}

func miseCommandCandidates() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "bin", executableName("mise")),
	}
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}

func prependPathDir(pathValue string, dir string) string {
	if dir == "" {
		return pathValue
	}
	for _, entry := range filepath.SplitList(pathValue) {
		if entry == dir {
			return pathValue
		}
	}
	if pathValue == "" {
		return dir
	}
	return dir + string(os.PathListSeparator) + pathValue
}
