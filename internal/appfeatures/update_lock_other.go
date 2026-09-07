//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package appfeatures

import "fmt"

func tryHostUpdateLock(string) (hostUpdateLock, error) {
	return nil, fmt.Errorf("standalone updates require an operating system file lock")
}
