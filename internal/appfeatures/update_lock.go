package appfeatures

type hostUpdateLock interface {
	release() error
}
