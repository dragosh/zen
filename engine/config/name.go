package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LegacyWorkspaceFile is the old name of the folder settings file. A folder
// that has it and no WorkspaceFile is still read; it is renamed the first time
// a folder setting is saved.
const LegacyWorkspaceFile = ".zen.yml"

// The notices a folder gets while it has the old file (Problems, the log and
// the command line say them).
const (
	legacyNotice = "This folder's settings are in " + LegacyWorkspaceFile + "; Zenmark now calls the file " + WorkspaceFile + "."
	bothNotice   = LegacyWorkspaceFile + " is ignored because " + WorkspaceFile + " exists."
	// LegacyFix is the help line for the legacy notice.
	LegacyFix = "It is renamed the next time a folder setting is saved, or rename it yourself."
)

// WorkspaceConfigPath is the one place that decides which settings file of
// the folder root is in use. With WorkspaceFile it is that file; with only
// LegacyWorkspaceFile it is the legacy file (legacy is true); with both it is
// WorkspaceFile and both is true; with neither it is the path to create.
func WorkspaceConfigPath(root string) (path string, legacy, both bool) {
	path = filepath.Join(root, WorkspaceFile)
	_, newErr := os.Lstat(path)
	oldPath := filepath.Join(root, LegacyWorkspaceFile)
	_, oldErr := os.Lstat(oldPath)
	switch {
	case newErr == nil && oldErr == nil:
		return path, false, true
	case newErr != nil && oldErr == nil:
		return oldPath, true, false
	}
	return path, false, false
}

// Notice is the text of the Problems item for a folder that still has the old
// settings file, or "" (see WorkspaceConfigPath).
func Notice(legacy, both bool) string {
	switch {
	case both:
		return bothNotice
	case legacy:
		return legacyNotice
	}
	return ""
}

// MoveLegacy renames the folder's legacy settings file to WorkspaceFile in
// place, keeping its bytes. It does nothing when there is no legacy file only
// (none, or both files). It refuses a link that leads out of the folder and
// anything that is not a regular file, and a failed rename leaves the legacy
// file as it was.
func MoveLegacy(root string) error {
	path, legacy, _ := WorkspaceConfigPath(root)
	if !legacy {
		return nil
	}
	if err := checkInside(root, path); err != nil {
		return err
	}
	if err := os.Rename(path, filepath.Join(root, WorkspaceFile)); err != nil {
		return fmt.Errorf("could not rename %s to %s: %w", LegacyWorkspaceFile, WorkspaceFile, errors.Unwrap(err))
	}
	return nil
}

// checkInside accepts a regular file of root, or a link in root to one.
func checkInside(root, path string) error {
	refuse := func(why string) error {
		return fmt.Errorf("will not rename %s: %s; rename or remove it yourself", LegacyWorkspaceFile, why)
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return refuse("it cannot be resolved")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return refuse("the folder cannot be resolved")
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return refuse("it is a link to a file outside the folder")
	}
	fi, err := os.Stat(real)
	if err != nil || !fi.Mode().IsRegular() {
		return refuse("it is not a file")
	}
	return nil
}
