package config

// Git for Windows' MSYS ssh ignores Forged's Include and can't open pipe agents.
type GitSSHStatus struct {
	Applicable bool
	Native     bool
	Source     string
	Fixable    bool
}

func (s GitSSHStatus) NeedsFix() bool {
	return s.Applicable && !s.Native && s.Source == "" && s.Fixable
}
