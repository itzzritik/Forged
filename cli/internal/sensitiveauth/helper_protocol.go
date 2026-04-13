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
}

const (
	helperTypeAuthorize = "authorize"
	helperTypeStatus    = "status"
	helperTypeSubscribe = "subscribe-locks"
	helperTypeEvent     = "event"

	helperStatusOK          = "ok"
	helperStatusCanceled    = "canceled"
	helperStatusUnavailable = "unavailable"
	helperStatusFailed      = "failed"

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
