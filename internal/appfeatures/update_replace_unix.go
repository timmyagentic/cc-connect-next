//go:build !windows

package appfeatures

import (
	"errors"
	"fmt"
	"os"
)

func createHostUpdateBackup(target, backup string) error {
	// Link creates the backup atomically without replacing another recovery
	// file, and keeps the executable available until the staged rename.
	if err := os.Link(target, backup); err != nil {
		return err
	}
	backupInfo, backupErr := os.Lstat(backup)
	targetInfo, targetErr := os.Lstat(target)
	if backupErr != nil || targetErr != nil || !backupInfo.Mode().IsRegular() ||
		!targetInfo.Mode().IsRegular() || !os.SameFile(backupInfo, targetInfo) {
		return errors.Join(fmt.Errorf("executable changed while creating update backup"), os.Remove(backup))
	}
	return nil
}

func syncHostUpdateDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	err = directory.Sync()
	return errors.Join(err, directory.Close())
}
