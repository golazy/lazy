package addonservice

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golazy.dev/lazycode"
)

var (
	renameAddonFile = os.Rename
	removeAddonFile = os.Remove
)

// Print writes a stable, compact changeset suitable for --dry-run output.
func (plan Plan) Print(output io.Writer) {
	if output == nil {
		return
	}
	if len(plan.Result.Files) == 0 && len(plan.ModeChanges) == 0 {
		fmt.Fprintln(output, "  no changes")
		return
	}
	for _, edit := range plan.Result.Files {
		fmt.Fprintf(output, "  would %s %s\n", edit.Kind, edit.Path)
	}
	for _, edit := range plan.ModeChanges {
		fmt.Fprintf(output, "  would chmod %o %s\n", edit.After, edit.Path)
	}
}

// Apply verifies every baseline before writing any file, stages replacements
// beside their targets, and then installs the planned changes.
func (plan Plan) Apply() error {
	if plan.Dir == "" {
		return errors.New("add-on plan root is required")
	}
	for _, edit := range plan.Result.Files {
		data, _, exists, err := readAddonTarget(plan.Dir, edit.Path)
		if err != nil {
			return fmt.Errorf("verify %s: %w", edit.Path, err)
		}
		if !edit.VerifyBaseline(data, exists) {
			return fmt.Errorf("%s changed after add-on planning; rerun the command", edit.Path)
		}
	}
	for _, edit := range plan.ModeChanges {
		data, mode, exists, err := readAddonTarget(plan.Dir, edit.Path)
		if err != nil {
			return fmt.Errorf("verify mode for %s: %w", edit.Path, err)
		}
		if !exists || lazycode.Hash(data) != edit.BaselineHash || uint32(mode.Perm()) != edit.Before {
			return fmt.Errorf("%s mode changed after add-on planning; rerun the command", edit.Path)
		}
	}

	type stagedFile struct {
		edit    lazycode.FileEdit
		path    string
		temp    string
		backup  string
		existed bool
	}
	staged := make([]stagedFile, 0, len(plan.Result.Files))
	cleanup := func() {
		for _, file := range staged {
			if file.temp != "" {
				_ = removeAddonFile(file.temp)
			}
			if file.backup != "" {
				_ = removeAddonFile(file.backup)
			}
		}
	}
	defer cleanup()

	stage := func(edit lazycode.FileEdit, requestedMode os.FileMode) error {
		if err := validateAddonTarget(plan.Dir, edit.Path); err != nil {
			return err
		}
		path := filepath.Join(plan.Dir, filepath.FromSlash(edit.Path))
		current, currentMode, exists, err := readAddonTarget(plan.Dir, edit.Path)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", edit.Path, err)
		}
		if !edit.VerifyBaseline(current, exists) {
			return fmt.Errorf("%s changed while staging add-on changes; rerun the command", edit.Path)
		}
		file := stagedFile{edit: edit, path: path, existed: exists}
		if edit.Kind != lazycode.EditDelete {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("create directory for %s: %w", edit.Path, err)
			}
			mode := requestedMode
			if mode == 0 {
				if exists {
					mode = currentMode
				} else {
					mode = 0o644
				}
			}
			temporary, err := os.CreateTemp(filepath.Dir(path), ".lazy-addon-*")
			if err != nil {
				return fmt.Errorf("stage %s: %w", edit.Path, err)
			}
			file.temp = temporary.Name()
			if err := temporary.Chmod(mode); err != nil {
				temporary.Close()
				return fmt.Errorf("set mode for %s: %w", edit.Path, err)
			}
			if _, err := temporary.Write(edit.After); err != nil {
				temporary.Close()
				return fmt.Errorf("stage %s: %w", edit.Path, err)
			}
			if err := temporary.Close(); err != nil {
				return fmt.Errorf("close staged %s: %w", edit.Path, err)
			}
		}
		staged = append(staged, file)
		return nil
	}

	for _, edit := range plan.Result.Files {
		if err := stage(edit, plan.Modes[edit.Path]); err != nil {
			return err
		}
	}
	for _, modeEdit := range plan.ModeChanges {
		data, mode, exists, err := readAddonTarget(plan.Dir, modeEdit.Path)
		if err != nil {
			return fmt.Errorf("stage mode for %s: %w", modeEdit.Path, err)
		}
		if !exists || lazycode.Hash(data) != modeEdit.BaselineHash || uint32(mode.Perm()) != modeEdit.Before {
			return fmt.Errorf("%s changed while staging its add-on mode; rerun the command", modeEdit.Path)
		}
		edit := lazycode.FileEdit{
			Path:         modeEdit.Path,
			Kind:         lazycode.EditUpdate,
			BaselineHash: modeEdit.BaselineHash,
			Before:       append([]byte(nil), data...),
			After:        append([]byte(nil), data...),
		}
		if err := stage(edit, os.FileMode(modeEdit.After)); err != nil {
			return err
		}
	}

	// addons.toml is the durable record of a successful apply, so it is always
	// installed after every source, task, and module output.
	sort.SliceStable(staged, func(i, j int) bool {
		if staged[i].edit.Path == StateFilename {
			return false
		}
		if staged[j].edit.Path == StateFilename {
			return true
		}
		return staged[i].edit.Path < staged[j].edit.Path
	})

	var committed []*stagedFile
	rollback := func(cause error) error {
		var rollbackErrors []string
		for index := len(committed) - 1; index >= 0; index-- {
			file := committed[index]
			if file.edit.Kind != lazycode.EditDelete {
				if err := removeAddonFile(file.path); err != nil && !errors.Is(err, os.ErrNotExist) {
					rollbackErrors = append(rollbackErrors, fmt.Sprintf("remove %s: %v", file.edit.Path, err))
				}
			}
			if file.backup != "" {
				if err := renameAddonFile(file.backup, file.path); err != nil {
					rollbackErrors = append(rollbackErrors, fmt.Sprintf("restore %s: %v", file.edit.Path, err))
				} else {
					file.backup = ""
				}
			}
		}
		if len(rollbackErrors) != 0 {
			return fmt.Errorf("%w; rollback failed: %s", cause, strings.Join(rollbackErrors, "; "))
		}
		return cause
	}

	for index := range staged {
		file := &staged[index]
		if err := validateAddonTarget(plan.Dir, file.edit.Path); err != nil {
			return rollback(err)
		}
		current, _, exists, err := readAddonTarget(plan.Dir, file.edit.Path)
		if err != nil {
			return rollback(fmt.Errorf("verify %s before install: %w", file.edit.Path, err))
		}
		if !file.edit.VerifyBaseline(current, exists) {
			return rollback(fmt.Errorf("%s changed during add-on apply; changes were rolled back", file.edit.Path))
		}
		if file.existed {
			backup, err := reserveBackup(filepath.Dir(file.path))
			if err != nil {
				return rollback(fmt.Errorf("prepare backup for %s: %w", file.edit.Path, err))
			}
			if err := renameAddonFile(file.path, backup); err != nil {
				_ = removeAddonFile(backup)
				return rollback(fmt.Errorf("back up %s: %w", file.edit.Path, err))
			}
			file.backup = backup
		}
		committed = append(committed, file)
		if file.edit.Kind == lazycode.EditDelete {
			continue
		}
		if err := renameAddonFile(file.temp, file.path); err != nil {
			return rollback(fmt.Errorf("install %s: %w", file.edit.Path, err))
		}
		file.temp = ""
	}
	for _, file := range committed {
		if file.backup != "" {
			if err := removeAddonFile(file.backup); err != nil {
				return fmt.Errorf("clean backup for %s: %w", file.edit.Path, err)
			}
			file.backup = ""
		}
	}
	return nil
}

func reserveBackup(dir string) (string, error) {
	file, err := os.CreateTemp(dir, ".lazy-addon-backup-*")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	if err := os.Remove(name); err != nil {
		return "", err
	}
	return name, nil
}
