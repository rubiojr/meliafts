package themes

// Synthwave: a warm "orchid sunset" — deep plum night with an analogous spread
// of orchid, magenta, rose, coral and gold. Deliberately free of any blue/cyan
// so the chrome and the rows stay harmonious.
func init() {
	register(Palette{
		Name:   "synthwave",
		Bg:     "#241327",
		BgAlt:  "#38203f",
		Fg:     "#f6e7f1",
		Hi:     "#fff4fb",
		Dim:    "#b884bb",
		Low:    "#6f476f",
		Accent: "#ff4fa3",
		Danger: "#ff5d6c",

		Sender:  "#c79bff",
		Subject: "#ff8fc7",
		Date:    "#cf8fd6",
		Unread:  "#ffcf5c",
		Attach:  "#ff9e7a",
		Label:   "#c79bff",

		Title:    "#ffcf5c",
		TitleAlt: "#ff4fa3",
		Bar:      "#c77be6",
		BarText:  "#1c0f20",
		Rule:     "#9a5fc4",
		Prompt:   "#ff8fc7",
	})
}
