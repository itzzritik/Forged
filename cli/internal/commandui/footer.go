package commandui

import "strings"

func FooterAction(key, label string) string {
	return "[" + key + "] " + label
}

func RenderFooter(actions ...string) string {
	if len(actions) == 0 {
		return ""
	}
	return strings.Join(actions, " "+MutedStyle.Render("·")+" ")
}
