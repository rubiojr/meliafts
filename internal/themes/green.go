package themes

// Green phosphor terminal: classic single-hue P1 green on black.
func init() {
	register(Palette{
		Name:   "green",
		Bg:     "#02160a",
		BgAlt:  "#06301a",
		Fg:     "#36ff7a",
		Hi:     "#c6ffd8",
		Dim:    "#1fb151",
		Low:    "#0f6a30",
		Accent: "#b6ff00",
		Danger: "#ff5c5c",
	})
}
