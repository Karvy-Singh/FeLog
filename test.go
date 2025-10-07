package main

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"

	"github.com/Karvy-Singh/FeLog/internals/tui"
	"github.com/codelif/katnip"
)

// --- 1) one handler per action ---------------------------------------------

// register a panel handler name that creates a TUI with its own label & onClick
func registerPanelHandler(handlerName, label, color string, onClick func()) {
	katnip.RegisterFunc(handlerName, func(k *katnip.Kitty, rw io.ReadWriter) int {
		t, err := tui.New(
			label,
			onClick,
			tui.WithCornerRadius(),    // your defaults
			tui.WithPanelColor(color), // unique color if you like
			// size/position come from katnip.Config in main via pixels
			// so we don't set WithSize/WithPosition here
		)
		if err != nil {
			return 1
		}
		return t.Run()
	})
}

func init() {
	// six distinct actions (wlogout-style)
	registerPanelHandler("FeLog:poweroff", "Power Off", "rgba(210, 15, 57, 1)", func() {
		_ = exec.Command("systemctl", "poweroff").Start()
	})
	registerPanelHandler("FeLog:reboot", "Reboot", "rgba(223, 142, 29, 1)", func() {
		_ = exec.Command("systemctl", "reboot").Start()
	})
	registerPanelHandler("FeLog:logout", "Log Out", "rgba(64, 160, 43, 1)", func() {
		_ = exec.Command("loginctl", "terminate-user", "$USER").Start()
	})
	registerPanelHandler("FeLog:suspend", "Suspend", "rgba(30, 102, 245, 1)", func() {
		_ = exec.Command("systemctl", "suspend").Start()
	})
	registerPanelHandler("FeLog:hibernate", "Hibernate", "rgba(136, 57, 239, 1)", func() {
		_ = exec.Command("systemctl", "hibernate").Start()
	})
	registerPanelHandler("FeLog:lock", "Lock", "rgba(49, 50, 68, 1)", func() {
		// swap for your locker
		_ = exec.Command("swaylock").Start()
	})
}

// --- 2) grid layout + spawn all six panels ----------------------------------

// getScreenSize should return the usable pixel width/height of the display.
// If katnip exposes an API for this, wire it here. Until then, defaults.
func getScreenSize() (int, int) {
	// TODO: replace with real query (e.g., from katnip) if available.
	const fallbackW, fallbackH = 1920, 1080
	return fallbackW, fallbackH
}

func main() {
	screenW, screenH := getScreenSize()

	// 3 columns × 2 rows grid, with padding/gap
	padding := 60 // outer margin around the whole grid
	gap := 40     // spacing between tiles
	cols, rows := 3, 2

	// compute square size that fits in screen with the chosen padding/gap
	availW := screenW - 2*padding - (cols-1)*gap
	availH := screenH - 2*padding - (rows-1)*gap
	tile := availW / cols
	if h := availH / rows; h < tile {
		tile = h
	}

	type item struct {
		handler string
		label   string
		color   string
		col     int // 0..2
		row     int // 0..1
	}

	items := []item{
		{"FeLog:poweroff", "Power Off", "rgba(210, 15, 57, 1)", 0, 0},
		{"FeLog:reboot", "Reboot", "rgba(223, 142, 29, 1)", 1, 0},
		{"FeLog:logout", "Log Out", "rgba(64, 160, 43, 1)", 2, 0},
		{"FeLog:suspend", "Suspend", "rgba(30, 102, 245, 1)", 0, 1},
		{"FeLog:hibernate", "Hibernate", "rgba(136, 57, 239, 1)", 1, 1},
		{"FeLog:lock", "Lock", "rgba(49, 50, 68, 1)", 2, 1},
	}

	// create & run all panels concurrently
	var wg sync.WaitGroup
	wg.Add(len(items))

	for _, it := range items {
		// compute top-left pixel of each tile
		x := padding + it.col*(tile+gap)
		y := padding + it.row*(tile+gap)

		cfg := katnip.Config{
			Edge:        katnip.EdgeNone,
			Position:    katnip.Vector{X: x, Y: y},
			Layer:       katnip.LayerTop,
			FocusPolicy: katnip.FocusOnDemand,
			KittyOverrides: []string{
				"background_opacity=0.0",
				"window_padding_width=0",
			},
			Overrides: map[string]string{
				"--lines":   fmt.Sprintf("%dpx", tile),
				"--columns": fmt.Sprintf("%dpx", tile),
			},
		}

		p := katnip.NewPanel(it.handler, cfg)

		go func(p *katnip.Panel, it item) {
			defer wg.Done()
			if err := p.Run(); err != nil {
				log.Printf("panel %s exited with error: %v", it.label, err)
			}
		}(p, it)
	}

	wg.Wait()
}
