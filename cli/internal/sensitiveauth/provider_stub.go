//go:build !darwin && !linux && !windows

package sensitiveauth

func NewNativeProvider() NativeProvider {
	return unavailableProvider{}
}
