package daemon

type ServiceStatus struct {
	Installed   bool
	ConfigValid bool
	Loaded      bool
	Running     bool
	Repairable  bool
	Detail      string
}

func DefaultServiceStatus() ServiceStatus {
	return ServiceStatus{Repairable: true}
}
