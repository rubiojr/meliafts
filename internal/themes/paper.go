package themes

// Paper: a warm light theme — fountain-pen inks on parchment. Dark sepia text
// on cream, with green/burgundy/sienna/ochre ink accents and coffee-brown
// chrome bars carrying cream text.
func init() {
	register(Palette{
		Name:   "paper",
		Bg:     "#f3e9d2",
		BgAlt:  "#e6d8ba",
		Fg:     "#4a3b28",
		Hi:     "#2a201a",
		Dim:    "#9a8a6a",
		Low:    "#b3a07e",
		Accent: "#c0532e",
		Danger: "#a52a2a",

		Sender:  "#356b4f",
		Subject: "#8a3b52",
		Label:   "#356b4f",
		Attach:  "#b5871d",

		Title:    "#5b3a1f",
		TitleAlt: "#7a5230",
		Bar:      "#6b4f2a",
		BarText:  "#f3e9d2",
		Rule:     "#c8b896",
		Prompt:   "#8a5a2a",
	})
}
