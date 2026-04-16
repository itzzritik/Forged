package tui

type RouteID string

const (
	RouteDashboardHome RouteID = "dashboard/home"
	RouteRepairTask    RouteID = "repair/task"
	RouteRepairResult  RouteID = "repair/result"

	RouteAccountLogin  RouteID = "account/login"
	RouteAccountLogout RouteID = "account/logout"
	RouteAccountStatus RouteID = "account/status"

	RouteSyncHome   RouteID = "sync/home"
	RouteSyncStatus RouteID = "sync/status"
	RouteSyncRun    RouteID = "sync/run"

	RouteKeysHome     RouteID = "keys/home"
	RouteKeysBrowser  RouteID = "keys/browser"
	RouteKeysDetail   RouteID = "keys/detail"
	RouteKeysRename   RouteID = "keys/rename"
	RouteKeysDelete   RouteID = "keys/delete"
	RouteKeysGenerate RouteID = "keys/generate"
	RouteKeysImport   RouteID = "keys/import"
	RouteKeysExport   RouteID = "keys/export"

	RouteVaultHome           RouteID = "vault/home"
	RouteVaultLock           RouteID = "vault/lock"
	RouteVaultUnlock         RouteID = "vault/unlock"
	RouteVaultChangePassword RouteID = "vault/change-password"

	RouteAgentHome    RouteID = "agent/home"
	RouteAgentEnable  RouteID = "agent/enable"
	RouteAgentDisable RouteID = "agent/disable"
	RouteAgentSigning RouteID = "agent/signing"

	RouteDoctorOverview RouteID = "doctor/overview"
)

type Intent struct {
	Entry       RouteID
	Boundary    RouteID
	CommandPath []string
	Args        []string
	Params      map[string]string
}

func NewIntent(entry RouteID) Intent {
	return Intent{
		Entry:    entry,
		Boundary: entry,
		Params:   map[string]string{},
	}
}

func (i Intent) WithCommandPath(path ...string) Intent {
	i.CommandPath = append([]string(nil), path...)
	return i
}

func (i Intent) WithArgs(args ...string) Intent {
	i.Args = append([]string(nil), args...)
	return i
}

func (i Intent) WithParam(key, value string) Intent {
	if i.Params == nil {
		i.Params = map[string]string{}
	}
	i.Params[key] = value
	return i
}

func (i Intent) Param(key string) string {
	if i.Params == nil {
		return ""
	}
	return i.Params[key]
}

func cloneParams(params map[string]string) map[string]string {
	if len(params) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}
