package main

import (
	"fmt"
	"io"

	"github.com/Karvy-Singh/FeLog/internals/tui"
	"github.com/codelif/katnip"
)

func panel(k *katnip.Kitty, rw io.ReadWriter) int {
	t, err := tui.New(
		tui.WithPosition(),
		tui.WithSize(),
		tui.WithCornerRadius(),
		tui.WithPanelColor(),
	)
	if err != nil {
		return 1
	}
	return t.Run() // handles raw mode, signals, icat, loop, cleanup
}

func init() {
	katnip.RegisterFunc("FeLog", panel)
}

func main() {
	p := katnip.NewPanel("FeLog", katnip.Config{
		Edge:        katnip.EdgeNone,
		Position:    katnip.Vector{X: 300, Y: 300},
		Layer:       katnip.LayerTop,
		FocusPolicy: katnip.FocusOnDemand,
		KittyOverrides: []string{
			"background_opacity=0.0",
			"window_padding_width=0",
		},
		Overrides: map[string]string{
			"--lines":   fmt.Sprintf("%dpx", 300),
			"--columns": fmt.Sprintf("%dpx", 300),
		},
	})
	p.Run()
}
