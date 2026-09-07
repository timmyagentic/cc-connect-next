package appfeatures

import "golang.org/x/sys/windows"

func createHostUpdateBackup(target, backup string) error {
	from, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(backup)
	if err != nil {
		return err
	}
	// A running Windows image must be renamed out of the way. Go's os.Rename
	// replaces an existing destination; MoveFile refuses to clobber a backup.
	return windows.MoveFile(from, to)
}

func syncHostUpdateDirectory(string) error { return nil }
