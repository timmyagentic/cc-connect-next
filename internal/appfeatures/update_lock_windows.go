package appfeatures

import (
	"errors"
	"fmt"

	featureupdater "github.com/timmyagentic/awesome-agent-app-features/updater"
	"golang.org/x/sys/windows"
)

type windowsHostUpdateLock struct{ handle windows.Handle }

func tryHostUpdateLock(path string) (hostUpdateLock, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("update lock path: %w", err)
	}
	// Deny sharing for the lifetime of the handle, including across CLI and
	// daemon processes. OPEN_ALWAYS does not truncate a pre-existing file.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil, windows.OPEN_ALWAYS, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return nil, featureupdater.ErrUpdateInProgress
	}
	if err != nil {
		return nil, fmt.Errorf("open update lock: %w", err)
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("inspect update lock: %w", err)
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("update lock is not a regular non-symlink file")
	}
	return &windowsHostUpdateLock{handle: handle}, nil
}

func (lock *windowsHostUpdateLock) release() error {
	if err := windows.CloseHandle(lock.handle); err != nil {
		return fmt.Errorf("close update lock: %w", err)
	}
	return nil
}
