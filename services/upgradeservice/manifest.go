package upgradeservice

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type upgradeFileAction string

const (
	upgradeFileAdd    upgradeFileAction = "add"
	upgradeFileUpdate upgradeFileAction = "update"
	upgradeFileDelete upgradeFileAction = "delete"
)

type upgradeFileManifest struct {
	From  string
	To    string
	Files []upgradeFileOperation
}

type upgradeFileOperation struct {
	Action   upgradeFileAction
	Path     string
	Mode     os.FileMode
	Previous upgradeFileContent
	Target   upgradeFileContent
}

type upgradeFileContent struct {
	Content string
	SHA256  string
}

func upgradeTo011Manifest() upgradeFileManifest {
	return upgradeFileManifest{
		From: "v0.1.10",
		To:   "v0.1.11",
		Files: []upgradeFileOperation{
			{
				Action: upgradeFileAdd,
				Path:   ".mise/tasks/dev",
				Mode:   0o755,
				Target: upgradeManifestContent(v011DevTask),
			},
			{
				Action: upgradeFileAdd,
				Path:   ".mise/tasks/test",
				Mode:   0o755,
				Target: upgradeManifestContent(v011TestTask),
			},
		},
	}
}

func upgradeManifestContent(content string) upgradeFileContent {
	data := []byte(content)
	return upgradeFileContent{
		Content: content,
		SHA256:  sha256Hex(data),
	}
}

func (e stepExecutor) applyFileManifest(manifest upgradeFileManifest) error {
	if manifest.From != "" && manifest.From != e.from {
		return fmt.Errorf("upgrade file manifest starts at %s, want %s", manifest.From, e.from)
	}
	if manifest.To != "" && manifest.To != e.to {
		return fmt.Errorf("upgrade file manifest targets %s, want %s", manifest.To, e.to)
	}
	for _, operation := range manifest.Files {
		if err := e.applyFileOperation(operation); err != nil {
			return err
		}
	}
	return nil
}

func (e stepExecutor) applyFileOperation(operation upgradeFileOperation) error {
	relative, err := cleanManifestPath(operation.Path)
	if err != nil {
		return err
	}
	switch operation.Action {
	case upgradeFileAdd:
		return e.applyAddFileOperation(relative, operation)
	case upgradeFileUpdate:
		return e.applyUpdateFileOperation(relative, operation)
	case upgradeFileDelete:
		return e.applyDeleteFileOperation(relative, operation)
	default:
		return fmt.Errorf("unknown upgrade file action %q for %s", operation.Action, relative)
	}
}

func (e stepExecutor) applyAddFileOperation(relative string, operation upgradeFileOperation) error {
	targetData, err := e.renderManifestContent(relative, "target", operation.Target)
	if err != nil {
		return err
	}
	mode := operation.fileMode()
	path := e.manifestPath(relative)
	data, err := os.ReadFile(path)
	if err == nil {
		if sameSHA256(data, targetData) {
			fmt.Fprintf(e.out(), "  %s already exists\n", relative)
			return nil
		}
		return e.installConflict(relative, data, targetData, mode)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", relative, err)
	}
	if e.dryRun {
		fmt.Fprintf(e.out(), "  would add %s\n", relative)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(relative), err)
	}
	if err := os.WriteFile(path, targetData, mode); err != nil {
		return fmt.Errorf("write %s: %w", relative, err)
	}
	fmt.Fprintf(e.out(), "  added %s\n", relative)
	return nil
}

func (e stepExecutor) applyUpdateFileOperation(relative string, operation upgradeFileOperation) error {
	previousData, err := e.renderManifestContent(relative, "previous", operation.Previous)
	if err != nil {
		return err
	}
	targetData, err := e.renderManifestContent(relative, "target", operation.Target)
	if err != nil {
		return err
	}
	mode := operation.fileMode()
	path := e.manifestPath(relative)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && e.bootstrapFromOlder {
			return e.applyAddFileOperation(relative, upgradeFileOperation{
				Action: upgradeFileAdd,
				Path:   relative,
				Mode:   mode,
				Target: operation.Target,
			})
		}
		return fmt.Errorf("read %s: %w", relative, err)
	}
	switch {
	case sameSHA256(data, targetData):
		fmt.Fprintf(e.out(), "  %s already matches %s\n", relative, e.to)
		return nil
	case sameSHA256(data, previousData):
		if e.dryRun {
			fmt.Fprintf(e.out(), "  would update %s\n", relative)
			return nil
		}
		if err := os.WriteFile(path, targetData, mode); err != nil {
			return fmt.Errorf("write %s: %w", relative, err)
		}
		fmt.Fprintf(e.out(), "  updated %s\n", relative)
		return nil
	default:
		return e.installConflict(relative, data, targetData, mode)
	}
}

func (e stepExecutor) applyDeleteFileOperation(relative string, operation upgradeFileOperation) error {
	previousData, err := e.renderManifestContent(relative, "previous", operation.Previous)
	if err != nil {
		return err
	}
	path := e.manifestPath(relative)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(e.out(), "  %s already removed\n", relative)
			return nil
		}
		return fmt.Errorf("read %s: %w", relative, err)
	}
	if sameSHA256(data, previousData) {
		if e.dryRun {
			fmt.Fprintf(e.out(), "  would remove %s\n", relative)
			return nil
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove %s: %w", relative, err)
		}
		fmt.Fprintf(e.out(), "  removed %s\n", relative)
		return nil
	}
	return e.deleteConflict(relative, data)
}

func (e stepExecutor) renderManifestContent(relative string, label string, content upgradeFileContent) ([]byte, error) {
	data := []byte(content.Content)
	if content.SHA256 != "" {
		if got := sha256Hex(data); got != content.SHA256 {
			return nil, fmt.Errorf("%s %s manifest hash mismatch: got sha256:%s, want sha256:%s", relative, label, got, content.SHA256)
		}
	}
	if e.modulePath != "" {
		data = bytes.ReplaceAll(data, []byte("sample_app"), []byte(e.modulePath))
	}
	return data, nil
}

func (e stepExecutor) installConflict(relative string, current []byte, proposed []byte, mode os.FileMode) error {
	diff := fullFileDiff(relative, current, proposed)
	if e.dryRun {
		return e.printConflictAndFail(relative, diff, proposed)
	}
	backupRelative := e.backupRelativePath(relative)
	install := false
	if err := e.takeover(func(stdin io.Reader, _ io.Writer, stderr io.Writer) error {
		fmt.Fprint(stderr, diff)
		fmt.Fprintf(stderr, "lazy: %s has local changes.\n", relative)
		fmt.Fprintf(stderr, "  [i] install the new version and back up the current file to %s\n", backupRelative)
		fmt.Fprintln(stderr, "  [a] abort so you can merge it manually")
		fmt.Fprint(stderr, "Choose an action [a]: ")
		answer, ok := readPromptAnswer(stdin)
		if !ok {
			fmt.Fprintln(stderr)
			return nil
		}
		install = answer == "i" || answer == "install"
		return nil
	}); err != nil {
		return err
	}
	if !install {
		return e.printConflictAndFail(relative, "", proposed)
	}
	actualBackup, err := e.backupCurrentFile(relative)
	if err != nil {
		return err
	}
	path := e.manifestPath(relative)
	if err := os.WriteFile(path, proposed, mode); err != nil {
		return fmt.Errorf("write %s: %w", relative, err)
	}
	fmt.Fprintf(e.out(), "  backed up %s to %s\n", relative, actualBackup)
	fmt.Fprintf(e.out(), "  installed new %s\n", relative)
	return nil
}

func (e stepExecutor) deleteConflict(relative string, current []byte) error {
	diff := deletedFileDiff(relative, current)
	if e.dryRun {
		return e.printConflictAndFail(relative, diff, nil)
	}
	backupRelative := e.backupRelativePath(relative)
	choice := ""
	if err := e.takeover(func(stdin io.Reader, _ io.Writer, stderr io.Writer) error {
		fmt.Fprint(stderr, diff)
		fmt.Fprintf(stderr, "lazy: %s has local changes, but the new sample app removes it.\n", relative)
		fmt.Fprintln(stderr, "Keeping it could create issues if the file is still loaded by your app.")
		fmt.Fprintf(stderr, "  [d] delete it and back up the current file to %s\n", backupRelative)
		fmt.Fprintln(stderr, "  [k] keep it and continue")
		fmt.Fprintln(stderr, "  [a] abort so you can decide manually")
		fmt.Fprint(stderr, "Choose an action [a]: ")
		answer, ok := readPromptAnswer(stdin)
		if !ok {
			fmt.Fprintln(stderr)
			return nil
		}
		choice = answer
		return nil
	}); err != nil {
		return err
	}
	switch choice {
	case "d", "delete":
		actualBackup, err := e.backupCurrentFile(relative)
		if err != nil {
			return err
		}
		if err := os.Remove(e.manifestPath(relative)); err != nil {
			return fmt.Errorf("remove %s: %w", relative, err)
		}
		fmt.Fprintf(e.out(), "  backed up %s to %s\n", relative, actualBackup)
		fmt.Fprintf(e.out(), "  removed %s\n", relative)
		return nil
	case "k", "keep":
		fmt.Fprintf(e.out(), "  kept %s; this could create issues if the file is still loaded by your app\n", relative)
		return nil
	default:
		return fmt.Errorf("upgrade conflict in %s; the new sample app removes this file, but the local file has changes", relative)
	}
}

func (e stepExecutor) printConflictAndFail(relative string, diff string, proposed []byte) error {
	if diff != "" {
		if err := e.takeover(func(_ io.Reader, _ io.Writer, stderr io.Writer) error {
			fmt.Fprint(stderr, diff)
			return nil
		}); err != nil {
			return err
		}
	}
	if !e.dryRun && proposed != nil {
		proposedPath := filepath.Join(e.dir, ".golazy", "upgrade", "conflicts", e.to, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(proposedPath), 0o755); err != nil {
			return fmt.Errorf("create conflict directory for %s: %w", relative, err)
		}
		if err := os.WriteFile(proposedPath, proposed, 0o644); err != nil {
			return fmt.Errorf("write proposed %s: %w", relative, err)
		}
	}
	return fmt.Errorf("upgrade conflict in %s; review the diff, edit the file, and rerun lazy upgrade", relative)
}

func (e stepExecutor) takeover(run func(stdin io.Reader, stdout io.Writer, stderr io.Writer) error) error {
	if e.ui != nil {
		return e.ui.Takeover(run)
	}
	return run(e.stdin, e.out(), e.errOut())
}

func (e stepExecutor) backupCurrentFile(relative string) (string, error) {
	path := e.manifestPath(relative)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s for backup: %w", relative, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("inspect %s for backup: %w", relative, err)
	}
	backupRelative, backupPath, err := e.availableBackupPath(relative)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return "", fmt.Errorf("create backup directory for %s: %w", relative, err)
	}
	if err := os.WriteFile(backupPath, data, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("write backup for %s: %w", relative, err)
	}
	return backupRelative, nil
}

func (e stepExecutor) availableBackupPath(relative string) (string, string, error) {
	base := e.backupRelativePath(relative)
	for index := 0; ; index++ {
		candidate := base
		if index > 0 {
			candidate = fmt.Sprintf("%s.%d", base, index)
		}
		path := e.manifestPath(candidate)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return candidate, path, nil
		} else if err != nil {
			return "", "", fmt.Errorf("inspect backup path %s: %w", candidate, err)
		}
	}
}

func (e stepExecutor) backupRelativePath(relative string) string {
	return relative + "-" + time.Now().Format("2006-01-02")
}

func (e stepExecutor) manifestPath(relative string) string {
	return filepath.Join(e.dir, filepath.FromSlash(relative))
}

func (e stepExecutor) out() io.Writer {
	if e.stdout == nil {
		return io.Discard
	}
	return e.stdout
}

func (e stepExecutor) errOut() io.Writer {
	if e.stderr == nil {
		return io.Discard
	}
	return e.stderr
}

func (operation upgradeFileOperation) fileMode() os.FileMode {
	if operation.Mode == 0 {
		return 0o644
	}
	return operation.Mode
}

func cleanManifestPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("upgrade file path is required")
	}
	if filepath.IsAbs(path) || strings.HasPrefix(filepath.ToSlash(path), "../") {
		return "", fmt.Errorf("upgrade file path %q must stay inside the app", path)
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("upgrade file path %q must stay inside the app", path)
	}
	return clean, nil
}

func readPromptAnswer(stdin io.Reader) (string, bool) {
	if stdin == nil {
		return "", false
	}
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(line)), true
}

func deletedFileDiff(relative string, current []byte) string {
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n", relative)
	fmt.Fprintln(&out, "+++ /dev/null")
	fmt.Fprintln(&out, "@@")
	for _, line := range splitLines(string(current)) {
		fmt.Fprintf(&out, "-%s\n", line)
	}
	return out.String()
}

func sameSHA256(data []byte, expected []byte) bool {
	return sha256Hex(data) == sha256Hex(expected)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}
