package ipc

import "time"

const (
	AccountChangeProtocol = 5

	SSHRoutePrepareWorkTimeout = 45 * time.Second
	SSHRoutePrepareCallTimeout = SSHRoutePrepareWorkTimeout + 5*time.Second
	ManualSyncCallTimeout      = 2 * time.Minute

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
	CmdSyncUnlink        = "sync-unlink"
	CmdAccountReplace    = "account-replace-v4"
	CmdAccountClear      = "account-clear-v4"
	CmdStatus            = "status"
	CmdSSHRoutePrepare   = "ssh-route-prepare"
	CmdSSHRouteSuccess   = "ssh-route-success"
	CmdSSHRouteSlot      = "ssh-route-slot"
	CmdSSHRoutesList     = "ssh-routes-list"
	CmdSSHRouteClear     = "ssh-route-clear"
	CmdSSHRoutesClearAll = "ssh-routes-clear-all"

	DefaultAPIServer = "https://forged-api.ritik.me"
	DefaultWebApp    = "https://forged.ritik.me"
)

type AccountCredentialsArgs struct {
	ServerURL        string    `json:"server_url"`
	Token            string    `json:"token,omitempty"`
	AccessToken      string    `json:"access_token,omitempty"`
	AccessExpiresAt  time.Time `json:"access_expires_at,omitempty"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at,omitempty"`
	UserID           string    `json:"user_id"`
	Email            string    `json:"email"`
	Name             string    `json:"name,omitempty"`
	ChangeID         string    `json:"change_id"`
}

type AccountChangeResult struct {
	SyncCleanupPending             bool `json:"sync_cleanup_pending,omitempty"`
	CredentialSecretCleanupPending bool `json:"credential_secret_cleanup_pending,omitempty"`
}

type SyncLinkArgs struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
}

type SSHRoutePrepareArgs struct {
	Attempt      string `json:"attempt"`
	ClientPID    int    `json:"client_pid"`
	CWD          string `json:"cwd"`
	Host         string `json:"host"`
	OriginalHost string `json:"original_host"`
	User         string `json:"user"`
	Port         string `json:"port"`
}

type SSHRouteSuccessArgs struct {
	Attempt   string `json:"attempt"`
	ClientPID int    `json:"client_pid"`
}

type SSHRouteSlotArgs struct {
	Attempt   string `json:"attempt"`
	ClientPID int    `json:"client_pid"`
	Slot      int    `json:"slot"`
}

type SSHRouteClearArgs struct {
	Target string `json:"target"`
}
