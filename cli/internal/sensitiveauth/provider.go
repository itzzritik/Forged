package sensitiveauth

import "errors"

var (
	ErrNativeUnavailable       = errors.New("System Auth unavailable")
	ErrNativeBroken            = errors.New("System Auth broken")
	ErrAuthenticationCanceled  = errors.New("Authentication canceled")
	ErrAuthenticationFailed    = errors.New("Authentication failed")
	ErrAuthorizationInProgress = errors.New("authentication already in progress")
	ErrLocked                  = errors.New("Forged is locked. Run `forged` to unlock it.")
)
