package tui

import "strings"

func DashboardIntent() Intent {
	return NewIntent(RouteDashboardHome)
}

func ResolveCommand(path []string, args []string) Intent {
	normalized := normalizeCommandPath(path)
	intent := DashboardIntent().WithCommandPath(normalized...).WithArgs(args...)

	switch strings.Join(normalized, " ") {
	case "":
		return intent
	case "login":
		return intentWithEntry(intent, RouteAccountLogin)
	case "logout":
		return intentWithEntry(intent, RouteAccountLogout)
	case "sync":
		return intentWithEntry(intent, RouteSyncRun)
	case "sync status":
		return intentWithEntry(intent, RouteSyncStatus)
	case "doctor":
		return intentWithEntry(intent, RouteDoctorOverview)
	case "key":
		return intentWithEntry(intent, RouteKeysBrowser)
	case "key list":
		return intentWithEntry(intent, RouteKeysBrowser)
	case "key view":
		intent = intentWithEntry(intent, RouteKeysDetail)
		if len(args) > 0 {
			intent = intent.WithParam("name", args[0])
		}
		return intent
	case "key rename":
		intent = intentWithEntry(intent, RouteKeysRename)
		if len(args) > 0 {
			intent = intent.WithParam("old_name", args[0])
		}
		if len(args) > 1 {
			intent = intent.WithParam("new_name", args[1])
		}
		return intent
	case "key delete":
		intent = intentWithEntry(intent, RouteKeysDelete)
		if len(args) > 0 {
			intent = intent.WithParam("name", args[0])
		}
		return intent
	case "key generate":
		intent = intentWithEntry(intent, RouteKeysGenerate)
		if len(args) > 0 {
			intent = intent.WithParam("name", args[0])
		}
		return intent
	case "key import":
		return intentWithEntry(intent, RouteKeysImport)
	case "key export":
		return intentWithEntry(intent, RouteKeysExport)
	case "vault":
		return intentWithEntry(intent, RouteVaultHome)
	case "vault lock":
		return intentWithEntry(intent, RouteVaultLock)
	case "vault unlock":
		return intentWithEntry(intent, RouteVaultUnlock)
	case "vault change-password":
		return intentWithEntry(intent, RouteVaultChangePassword)
	case "agent":
		return intentWithEntry(intent, RouteAgentHome)
	case "agent enable":
		return intentWithEntry(intent, RouteAgentEnable)
	case "agent disable":
		return intentWithEntry(intent, RouteAgentDisable)
	case "agent signing":
		intent = intentWithEntry(intent, RouteAgentSigning)
		if len(args) > 0 {
			intent = intent.WithParam("name", args[0])
		}
		return intent
	default:
		return intent
	}
}

func intentWithEntry(intent Intent, entry RouteID) Intent {
	intent.Entry = entry
	intent.Boundary = entry
	return intent
}

func normalizeCommandPath(path []string) []string {
	if len(path) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(path))
	for _, part := range path {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || trimmed == "forged" {
			continue
		}
		normalized = append(normalized, trimmed)
	}

	return normalized
}
