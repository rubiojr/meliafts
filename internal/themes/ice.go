package themes

// Ice: a cool but varied spread over deep navy. Sky/cyan chrome frame, steel
// rules, glacial result columns.
func init() {
	register(Palette{
		Name:   "ice",
		Bg:     "#03121f",
		BgAlt:  "#0a2942",
		Fg:     "#8fd6ff",
		Hi:     "#eaf7ff",
		Dim:    "#5aa6d6",
		Low:    "#2f6a8f",
		Accent: "#66f0ff",
		Danger: "#ff7a8a",

		Sender:  "#7ee8c8",
		Subject: "#c9b3ff",
		Date:    "#9ad0ff",
		Unread:  "#eaf7ff",
		Attach:  "#a6f0c6",
		Label:   "#7ee8c8",

		Title:    "#66f0ff",
		TitleAlt: "#8fd6ff",
		Bar:      "#7ee8c8",
		BarText:  "#03121f",
		Rule:     "#4f9fd0",
		Prompt:   "#7ee8c8",
	})
}
