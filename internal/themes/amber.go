package themes

// Amber phosphor CRT: a single warm hue on near-black. Monochrome by design.
func init() {
	register(Palette{
		Name:   "amber",
		Bg:     "#160d00",
		BgAlt:  "#2c1b04",
		Fg:     "#ffb000",
		Hi:     "#ffd982",
		Dim:    "#c08218",
		Low:    "#7a5410",
		Accent: "#ff8a1e",
		Danger: "#ff5436",
	})
}
