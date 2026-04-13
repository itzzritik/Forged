//go:build darwin

package sensitiveauth

func NewNativeProvider() NativeProvider {
	return unavailableProvider{}
}
