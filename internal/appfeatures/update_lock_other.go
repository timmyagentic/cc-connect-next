//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package appfeatures

type processOnlyHostUpdateLock struct{}

func tryHostUpdateLock(string) (hostUpdateLock, error) {
	return processOnlyHostUpdateLock{}, nil
}

func (processOnlyHostUpdateLock) release() error { return nil }
