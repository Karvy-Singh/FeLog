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
	cols     = 3
	rows     = 2
	tileSize = 310
)

func main() {
	screenW := flag.Int("screen-w", 1920, "screen width in px")
	screenH := flag.Int("screen-h", 1080, "screen height in px")
	gap := flag.Int("gap", 100, "tile gap in px")
	flag.Parse()

	totalW := cols*tileSize + (cols-1)*(*gap)
	totalH := rows*tileSize + (rows-1)*(*gap)
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
		col     int
		row     int
		clickFn func()
	}

	items := []item{
		{"FeLog:shutdown", "shutdown", 0, 0, func() { _ = exec.Command("systemctl", "poweroff").Start() }},
		{"FeLog:reboot", "reboot", 1, 0, func() { _ = exec.Command("systemctl", "reboot").Start() }},
		{"FeLog:logout", "logout", 2, 0, func() { _ = exec.Command("loginctl", "terminate-user", os.Getenv("USER")).Start() }},
		{"FeLog:suspend", "suspend", 0, 1, func() { _ = exec.Command("systemctl", "suspend").Start() }},
		{"FeLog:hibernate", "hibernate", 1, 1, func() { _ = exec.Command("systemctl", "hibernate").Start() }},
		{"FeLog:lock", "lock", 2, 1, func() { _ = exec.Command("loginctl", "lock-session").Start() }},
	}

	// register 6 handlers (each builds its own TUI with its own color/label/click)
	for _, it := range items {
		handlerName := it.handler
		label := it.label
		onClick := it.clickFn

		katnip.RegisterFunc(handlerName, func(k *katnip.Kitty, rw io.ReadWriter) int {
			t, err := tui.New(label, onClick,
				tui.WithSize(),
				tui.WithCornerRadius(),
				tui.WithPanelColor(),
			)
			if err != nil {
				return 1
			}
			return t.Run()
		})
	}

	// spawn all panels concurrently
	type paneMeta struct {
		label string
		do    func() error
	}
	panelMeta := map[*katnip.Panel]paneMeta{}

	wrap := func(cmd *exec.Cmd) func() error {
		return func() error {
			// capture errors; don’t hide them
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(items))

	var once sync.Once
	var panels []*katnip.Panel

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
			Overrides: map[string]string{
				"--lines":   fmt.Sprintf("%dpx", tileSize),
				"--columns": fmt.Sprintf("%dpx", tileSize),
			},
		}

		p := katnip.NewPanel(it.handler, cfg)
		panels = append(panels, p)

		// build parent-side action (with error reporting)
		var do func() error
		switch it.label {
		case "shutdown":
			do = wrap(exec.Command("systemctl", "poweroff"))
		case "reboot":
			do = wrap(exec.Command("systemctl", "reboot"))
		case "logout":
			do = wrap(exec.Command("loginctl", "terminate-user", os.Getenv("USER")))
		case "suspend":
			do = wrap(exec.Command("systemctl", "suspend"))
		case "hibernate":
			do = wrap(exec.Command("systemctl", "hibernate"))
		case "lock":
			do = wrap(exec.Command("loginctl", "lock-session"))
		default:
			do = func() error { return nil }
		}

		panelMeta[p] = paneMeta{label: it.label, do: do}

		go func(lbl string, p *katnip.Panel) {
			defer wg.Done()
			if err := p.Run(); err != nil {
				log.Printf("panel %s exited with error: %v", lbl, err)
			}

			// On first panel to exit, detect if it was a click and act
			once.Do(func() {
				clickedFlag := fmt.Sprintf("/tmp/felog_clicked_%s", lbl)
				_, clicked := os.Stat(clickedFlag)
				if clicked == nil {
					_ = os.Remove(clickedFlag)
					if meta, ok := panelMeta[p]; ok && meta.do != nil {
						if err := meta.do(); err != nil {
							log.Printf("action %s failed: %v", meta.label, err)
						}
					}
				} else {
					log.Printf("panel %s exited without click (q/close)", lbl)
				}

				// kill the rest
				for _, other := range panels {
					if other != p {
						_ = other.Kill()
					}
				}
			})
		}(it.label, p)
	}

	wg.Wait()
}
