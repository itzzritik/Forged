package ui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
)

func HuhTheme() huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles {
		st := huh.ThemeBase(Dark())
		p := P()
		for _, f := range []*huh.FieldStyles{&st.Focused, &st.Blurred} {
			f.Base = f.Base.UnsetBorderStyle().BorderLeft(false).PaddingLeft(0)
			f.Title = Fg(p.Muted)
			f.Description = Fg(p.Faint)
			f.ErrorIndicator = Fg(p.Danger)
			f.ErrorMessage = Fg(p.Danger)
			f.SelectSelector = Fg(p.Accent)
			f.Option = Fg(p.Text)
			f.SelectedOption = Fg(p.Text).Bold(true)
			f.TextInput.Cursor = Fg(p.Accent)
			f.TextInput.Placeholder = Fg(p.Faint)
			f.TextInput.Prompt = Fg(p.Accent)
			f.TextInput.Text = Fg(p.Text)
			f.FocusedButton = Fg(p.OnAccent).Background(p.Accent).Bold(true).Padding(0, 2)
			f.BlurredButton = Fg(p.Text).Background(p.Cap).Padding(0, 2)
		}
		return st
	})
}

func HuhKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Input.Prev = key.NewBinding(key.WithKeys("up"))
	km.Select.Prev = key.NewBinding(key.WithKeys("up"))
	km.Quit = key.NewBinding(key.WithKeys("esc"))
	return km
}
