//go:build windows

package platform

func Mlock(b []byte) error {
	return nil
}

func Munlock(b []byte) error {
	return nil
}
