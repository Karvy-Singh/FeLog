package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

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
	}

	items := []item{
		{"FeLog:shutdown", "shutdown", 0, 0},
		{"FeLog:reboot", "reboot", 1, 0},
		{"FeLog:logout", "logout", 2, 0},
		{"FeLog:suspend", "suspend", 0, 1},
		{"FeLog:hibernate", "hibernate", 1, 1},
		{"FeLog:lock", "lock", 2, 1},
	}

	// channel that the parent will use internally once it sees a click file
	clickCh := make(chan string, 1)

	// register handlers: each TUI writes /tmp/felog_clicked_<label> on click
	for _, it := range items {
		handlerName := it.handler
		label := it.label

		katnip.RegisterFunc(handlerName, func(k *katnip.Kitty, rw io.ReadWriter) int {
			clickFn := func() {
				path := fmt.Sprintf("/tmp/felog_clicked_%s", label)
				f, _ := os.Create(path)
				if f != nil {
					_ = f.Close()
				}
			}

			t, err := tui.New(label, clickFn,
				tui.WithSize(),
				tui.WithCornerRadius(),
				tui.WithPanelColor(),
			)
			if err != nil {
				return 1
			}
			err = k.Show()
			if err != nil {
				return 1
			}
			return t.Run()
		})
	}

	type paneMeta struct {
		label string
		do    func() error
	}

	panelMeta := map[string]paneMeta{}

	wrap := func(cmd *exec.Cmd) func() error {
		return func() error {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(items))

	var once sync.Once
	var panels []*katnip.Panel
	var panelReady sync.WaitGroup // Synchronization for panel readiness
	panelReady.Add(len(items))    // Add all panels to "ready" count

	for _, it := range items {
		x := paddingX + it.col*(tileSize+*gap)
		y := paddingY + it.row*(tileSize+*gap)

		cfg := katnip.Config{
			Edge:          katnip.EdgeNone,
			Position:      katnip.Vector{X: x, Y: y},
			Layer:         katnip.LayerTop,
			FocusPolicy:   katnip.FocusOnDemand,
			StartAsHidden: true,
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

		panelMeta[it.label] = paneMeta{label: it.label, do: do}

		// Initialize each panel in goroutines
		go func(lbl string, p *katnip.Panel) {
			defer wg.Done()
			defer panelReady.Done() // Mark the panel as "ready" when initialization finishes
			if err := p.Run(); err != nil {
				log.Printf("panel %s exited with error: %v", lbl, err)
			} else {
				log.Printf("panel %s exited cleanly", lbl)
			}
		}(it.label, p)
	}

	panelReady.Wait()

	// watcher: looks for /tmp/felog_clicked_<label> written by any TUI
	go func() {
		labels := make([]string, 0, len(items))
		for _, it := range items {
			labels = append(labels, it.label)
		}

		for {
			for _, lbl := range labels {
				path := fmt.Sprintf("/tmp/felog_clicked_%s", lbl)
				if _, err := os.Stat(path); err == nil {
					_ = os.Remove(path)
					select {
					case clickCh <- lbl:
					default:
					}
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// react to the first click: run action and kill all panels together
	go func() {
		lbl := <-clickCh
		once.Do(func() {
			if meta, ok := panelMeta[lbl]; ok && meta.do != nil {
				if err := meta.do(); err != nil {
					log.Printf("action %s failed: %v", meta.label, err)
				}
			} else {
				log.Printf("no action registered for label %s", lbl)
			}

			for _, p := range panels {
				_ = p.Kill()
			}
		})
	}()

	wg.Wait()
}
