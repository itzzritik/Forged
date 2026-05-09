package tui

type RouteID string

const (
	RouteDashboardHome RouteID = "dashboard/home"

	RouteAccountLogin  RouteID = "account/login"
	RouteAccountStatus RouteID = "account/status"

	RouteSyncHome RouteID = "sync/home"

	RouteKeysBrowser  RouteID = "keys/browser"
	RouteKeysDetail   RouteID = "keys/detail"
	RouteKeysRename   RouteID = "keys/rename"
	RouteKeysDelete   RouteID = "keys/delete"
	RouteKeysGenerate RouteID = "keys/generate"
	RouteKeysImport   RouteID = "keys/import"
	RouteKeysExport   RouteID = "keys/export"

	RouteVaultHome                   RouteID = "vault/home"
	RouteVaultLock                   RouteID = "vault/lock"
	RouteVaultUnlock                 RouteID = "vault/unlock"
	RouteVaultMasterPasswordInterval RouteID = "vault/master-password-interval"
	RouteVaultChangePassword         RouteID = "vault/change-password"

	RouteAgentHome    RouteID = "agent/home"
	RouteAgentSigning RouteID = "agent/signing"
	RouteAgentRouting RouteID = "agent/routing"

	RouteDoctorOverview RouteID = "doctor/overview"
)

func DashboardIntent() Intent {
	return NewIntent(RouteDashboardHome)
}

type Intent struct {
	Entry    RouteID
	Boundary RouteID
	Params   map[string]string
}

func NewIntent(entry RouteID) Intent {
	return Intent{
		Entry:    entry,
		Boundary: entry,
		Params:   map[string]string{},
	}
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
