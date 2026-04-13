//go:build windows

package sensitiveauth

func NewNativeProvider() NativeProvider {
	return unavailableProvider{}
}
