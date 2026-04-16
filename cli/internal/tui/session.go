package tui

type Route struct {
	ID     RouteID
	Params map[string]string
}

type EscMode string

const (
	EscAuto        EscMode = "auto"
	EscCancel      EscMode = "cancel"
	EscCloseSearch EscMode = "close-search"
)

type Session struct {
	stack    []Route
	boundary int
}

func NewSession(intent Intent) *Session {
	return &Session{
		stack: []Route{{
			ID:     intent.Entry,
			Params: cloneParams(intent.Params),
		}},
		boundary: 0,
	}
}

func (s *Session) Current() Route {
	if s == nil || len(s.stack) == 0 {
		return Route{}
	}
	return s.stack[len(s.stack)-1]
}

func (s *Session) Push(route Route) {
	if s == nil {
		return
	}
	s.stack = append(s.stack, Route{
		ID:     route.ID,
		Params: cloneParams(route.Params),
	})
}

func (s *Session) CanGoBack() bool {
	if s == nil {
		return false
	}
	return len(s.stack)-1 > s.boundary
}

func (s *Session) Back() bool {
	if !s.CanGoBack() {
		return false
	}
	s.stack = s.stack[:len(s.stack)-1]
	return true
}

func (s *Session) EscLabel(mode EscMode) string {
	switch mode {
	case EscCancel:
		return "Cancel"
	case EscCloseSearch:
		return "Close Search"
	default:
		if s.CanGoBack() {
			return "Back"
		}
		return "Exit"
	}
}
