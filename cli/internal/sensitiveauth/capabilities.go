package sensitiveauth

type CapabilityState string

const (
	CapabilityAvailable             CapabilityState = "available"
	CapabilityUnavailableByPlatform CapabilityState = "unavailable_by_platform"
	CapabilityUnavailableByEnv      CapabilityState = "unavailable_by_environment"
	CapabilityBroken                CapabilityState = "broken"
)

func (s CapabilityState) IsAvailable() bool {
	return s == CapabilityAvailable
}
