package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/Karvy-Singh/FeLog/internals/tui"
	"github.com/codelif/katnip"
)

const (
	cols = 3
	rows = 2
	// IMPORTANT: This is the image size your TUI renders (defaults to 300×300
	// because tui.WithSize() is fixed). We keep the panel the same size so
	// image == panel and there’s no stretching or clipping.
	tileSize = 310
)

func main() {
	// --- manual screen size (no katnip API) ---
	screenW := flag.Int("screen-w", 1920, "screen width in px")
	screenH := flag.Int("screen-h", 1080, "screen height in px")
	gap := flag.Int("gap", 100, "tile gap in px")
	flag.Parse()

	totalW := cols*tileSize + (cols-1)**gap
	totalH := rows*tileSize + (rows-1)**gap
	paddingX := (*screenW - totalW) / 2
	paddingY := (*screenH - totalH) / 2
	if paddingX < 0 {
		paddingX = 0
	}
	if paddingY < 0 {
		paddingY = 0
	}

	type item struct {
		handler string
		label   string
		color   string
		col     int
		row     int
		clickFn func()
	}

	// single-shot exit: once any tile is clicked, exit the whole program
	exitAfterClick := func(f func()) func() {
		return func() {
			// run the action (non-blocking where it makes sense)
			if f != nil {
				_ = exec.Command("sh", "-lc", "true").Run() // ensure shell available
				f()
			}
			// kill this app after one click (wlogout behavior)
			os.Exit(0)
		}
	}

	items := []item{
		{"FeLog:poweroff", "Power Off", "rgba(210, 15, 57, 1)", 0, 0, func() { _ = exec.Command("systemctl", "poweroff").Start() }},
		{"FeLog:reboot", "Reboot", "rgba(223, 142, 29, 1)", 1, 0, func() { _ = exec.Command("systemctl", "reboot").Start() }},
		{"FeLog:logout", "Log Out", "rgba(64, 160, 43, 1)", 2, 0, func() { _ = exec.Command("loginctl", "terminate-user", os.Getenv("USER")).Start() }},
		{"FeLog:suspend", "Suspend", "rgba(30, 102, 245, 1)", 0, 1, func() { _ = exec.Command("systemctl", "suspend").Start() }},
		{"FeLog:hibernate", "Hibernate", "rgba(136, 57, 239, 1)", 1, 1, func() { _ = exec.Command("systemctl", "hibernate").Start() }},
		{"FeLog:lock", "Lock", "rgba(49, 50, 68, 1)", 2, 1, func() { _ = exec.Command("swaylock").Start() }}, // swap your locker
	}

	// register 6 handlers (each builds its own TUI with its own color/label/click)
	for _, it := range items {
		handlerName := it.handler
		label := it.label
		color := it.color
		onClick := exitAfterClick(it.clickFn)

		katnip.RegisterFunc(handlerName, func(k *katnip.Kitty, rw io.ReadWriter) int {
			// We call WithSize() to lock TUI image at 300×300.
			// Panel size is also 300×300, so they match exactly.
			t, err := tui.New(label, onClick,
				tui.WithSize(),
				tui.WithCornerRadius(),
				tui.WithPanelColor(color),
			)
			if err != nil {
				return 1
			}
			return t.Run()
		})
	}

	// spawn all panels concurrently
	var wg sync.WaitGroup
	wg.Add(len(items))

	for _, it := range items {
		x := paddingX + it.col*(tileSize+*gap)
		y := paddingY + it.row*(tileSize+*gap)

		cfg := katnip.Config{
			Edge:        katnip.EdgeNone,
			Position:    katnip.Vector{X: x, Y: y},
			Layer:       katnip.LayerTop,
			FocusPolicy: katnip.FocusOnDemand,
			KittyOverrides: []string{
				"background_opacity=0.0",
				"window_padding_width=0",
			},
			// IMPORTANT: keep panel size == image size (300px)
			Overrides: map[string]string{
				"--lines":   fmt.Sprintf("%dpx", tileSize),
				"--columns": fmt.Sprintf("%dpx", tileSize),
			},
		}

		p := katnip.NewPanel(it.handler, cfg)
		go func(lbl string) {
			defer wg.Done()
			if err := p.Run(); err != nil {
				log.Printf("panel %s exited with error: %v", lbl, err)
			}
		}(it.label)
	}

	wg.Wait()
}
