package daemon

type ServiceStatus struct {
	Installed     bool
	ConfigValid   bool
	Loaded        bool
	Running       bool
	PID           int
	Repairable    bool
	Detail        string
	BinaryPath    string
	BinaryMissing bool
}

func DefaultServiceStatus() ServiceStatus {
	return ServiceStatus{Repairable: true}
}
