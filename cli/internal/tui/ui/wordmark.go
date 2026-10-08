package ui

func Wordmark() []string {
	if ASCII() {
		return []string{"", Bold("F O R G E D", P().Text), ""}
	}
	rows := []string{
		"█▀▀ █▀█ █▀█ █▀▀▀ █▀▀ █▀▄",
		"█▀  █ █ █▀▄ █ ▀█ █▀▀ █ █",
		"▀   ▀▀▀ ▀ ▀ ▀▀▀▀ ▀▀▀ ▀▀ ",
	}
	for i, r := range rows {
		rows[i] = Paint(r, P().Text)
	}
	return rows
}
