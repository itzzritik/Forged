package ipc

const (
	CmdList              = "list"
	CmdAdd               = "add"
	CmdGenerate          = "generate"
	CmdRemove            = "remove"
	CmdRename            = "rename"
	CmdExport            = "export"
	CmdView              = "view"
	CmdExportAll         = "export-all"
	CmdSensitiveAuth     = "sensitive-auth"
	CmdSensitivePassword = "sensitive-password"
	CmdSensitiveLock     = "sensitive-lock"
	CmdActivity          = "activity"
	CmdSyncTrigger       = "sync-trigger"
	CmdSyncLink          = "sync-link"
	CmdStatus            = "status"
	CmdSSHRoutePrepare   = "ssh-route-prepare"
	CmdSSHRouteSuccess   = "ssh-route-success"

	DefaultAPIServer = "https://forged-api.ritik.me"
	DefaultWebApp    = "https://forged.ritik.me"
)

type SyncLinkArgs struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
}

type SSHRoutePrepareArgs struct {
	Attempt   string `json:"attempt"`
	ClientPID int    `json:"client_pid"`
	CWD       string `json:"cwd"`
	Host      string `json:"host"`
	User      string `json:"user"`
	Port      string `json:"port"`
}

type SSHRouteSuccessArgs struct {
	Attempt string `json:"attempt"`
}
