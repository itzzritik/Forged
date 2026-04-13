package ipc

const (
	CmdList              = "list"
	CmdAdd               = "add"
	CmdGenerate          = "generate"
	CmdRemove            = "remove"
	CmdRename            = "rename"
	CmdExport            = "export"
	CmdView              = "view"
	CmdHost              = "host"
	CmdUnhost            = "unhost"
	CmdHosts             = "hosts"
	CmdExportAll         = "export-all"
	CmdSensitiveAuth     = "sensitive-auth"
	CmdSensitivePassword = "sensitive-password"
	CmdSensitiveLock     = "sensitive-lock"
	CmdActivity          = "activity"
	CmdSyncTrigger       = "sync-trigger"
	CmdSyncLink          = "sync-link"
	CmdStatus            = "status"

	DefaultAPIServer = "https://forged-api.ritik.me"
	DefaultWebApp    = "https://forged.ritik.me"
)

type SyncLinkArgs struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
}
