package sensitiveauth

type HelperRequest struct {
	ID     string `json:"id,omitempty"`
	Type   string `json:"type"`
	Action string `json:"action,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type HelperResponse struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type"`
	Status   string `json:"status,omitempty"`
	Provider string `json:"provider,omitempty"`
	Message  string `json:"message,omitempty"`
	// Secret carries the base64 master password from a collect-password popup.
	// ponytail: it lands in an immutable Go string that can't be zeroed; the
	// decoded []byte is wiped, the string lingers until GC. Same exposure class
	// as the TUI shipping the password over its socket. Never log this field.
	Secret string `json:"secret,omitempty"`
}

const (
	helperTypeAuthorize       = "authorize"
	helperTypeCancel          = "cancel"
	helperTypeCollectPassword = "collect-password"
	helperTypeStatus          = "status"
	helperTypeSubscribe       = "subscribe-locks"
	helperTypeEvent           = "event"

	helperStatusOK                  = "ok"
	helperStatusCanceled            = "canceled"
	helperStatusUnavailable         = "unavailable"
	helperStatusUnavailablePlatform = "unavailable_by_platform"
	helperStatusUnavailableEnv      = "unavailable_by_environment"
	helperStatusBroken              = "broken"
	helperStatusFailed              = "failed"

	helperEventSessionLocked = "session_locked"
)

func NewAuthorizeRequest(id string, action Action) HelperRequest {
	return HelperRequest{
		ID:     id,
		Type:   helperTypeAuthorize,
		Action: string(action),
		Reason: action.NativeReason(),
	}
}

func NewCancelRequest(id string) HelperRequest {
	return HelperRequest{
		ID:   id,
		Type: helperTypeCancel,
	}
}

func NewCollectPasswordRequest(id, reason string) HelperRequest {
	return HelperRequest{
		ID:     id,
		Type:   helperTypeCollectPassword,
		Reason: reason,
	}
}

func NewSubscribeLocksRequest(id string) HelperRequest {
	return HelperRequest{
		ID:   id,
		Type: helperTypeSubscribe,
	}
}

func NewStatusRequest(id string) HelperRequest {
	return HelperRequest{
		ID:   id,
		Type: helperTypeStatus,
	}
}
