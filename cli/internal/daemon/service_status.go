package daemon

type ServiceStatus struct {
	Installed        bool
	ConfigValid      bool
	Loaded           bool
	Running          bool
	PID              int
	PIDKnown         bool
	OwnershipBlocked bool
	Repairable       bool
	Detail           string
	BinaryPath       string
	BinaryMissing    bool
}

type LingerState struct {
	Applies, On bool
	User        string
}

func DefaultServiceStatus() ServiceStatus {
	return ServiceStatus{Repairable: true}
}

func setServiceDetail(status *ServiceStatus, detail string) {
	if status.Detail == "" {
		status.Detail = detail
	}
}

func invalidateServiceConfig(status *ServiceStatus, detail string) {
	status.ConfigValid = false
	setServiceDetail(status, detail)
}
