package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/codelif/katnip"
	"github.com/gdamore/tcell/v2"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func makeRoundedPNG(out string, width, height, radius int, rgba string) error {
	cmd := exec.Command(
		"magick",
		"-size", fmt.Sprintf("%dx%d", width, height), "canvas:none",
		"-fill", rgba,
		"-draw", fmt.Sprintf("roundrectangle 0,0 %d,%d %d,%d", width-1, height-1, radius, radius),
		"PNG32:"+out,
	)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func panelWithTcell(k *katnip.Kitty, rw io.ReadWriter) int {
	screen, err := tcell.NewScreen()
	must(err)
	defer screen.Fini()

	err = screen.Init()
	must(err)

	screen.Clear()
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack))

	width, height := 300, 300
	radius := 10
	color := tcell.ColorBlue

	for x := radius; x < width-radius; x++ {
		for y := radius; y < height-radius; y++ {
			screen.SetContent(x, y, ' ', nil, tcell.StyleDefault.Foreground(color))
		}
	}

	cols := os.Getenv("COLUMNS")
	lines := os.Getenv("LINES")
	if cols == "" {
		cols = "100"
	}
	if lines == "" {
		lines = "40"
	}

	place := fmt.Sprintf("%sx%s@0x0", cols, lines)

	icat := exec.Command("kitty", "+kitten", "icat",
		"--stdin=no",
		"--use-window-size", fmt.Sprintf("%s,%s,%d,%d", cols, lines, width, height),
		"--place", place,
		"-z", "-1",
		"--background=none",
		"/tmp/kitty_panel_rounded.png",
	)
	icat.Env = os.Environ()
	icat.Stdout, icat.Stderr = os.Stdout, os.Stderr
	_ = icat.Run()

	fmt.Println("\n\nhello world")
	fmt.Printf("\033[?25l")
	for {
		event := screen.PollEvent()
		switch e := event.(type) {
		case *tcell.EventKey:
			if e.Key() == tcell.KeyEscape {
				return 0
			}
		case *tcell.EventResize:
			width, height = screen.Size()
			screen.Clear()
		}
	}
}

func init() {
	katnip.RegisterFunc("rounded-demo", panelWithTcell)
}

func main() {
	width, height := 300, 300
	radius := 10
	color := "#4A708B"
	out := "/tmp/kitty_panel_rounded.png"
	must(makeRoundedPNG(out, width, height, radius, color))

	cfg := katnip.Config{
		Edge:        katnip.EdgeNone,
		Position:    katnip.Vector{X: 300, Y: 300},
		Layer:       katnip.LayerTop,
		FocusPolicy: katnip.FocusNotAllowed,
		KittyOverrides: []string{
			"background_opacity=0.0",
			"window_padding_width=0",
			fmt.Sprintf("cursor=%s", color),
		},
		Overrides: map[string]string{
			"--lines":   fmt.Sprintf("%dpx", height),
			"--columns": fmt.Sprintf("%dpx", width),
		},
	}

	p := katnip.NewPanel("rounded-demo", cfg)
	p.Run()

	time.Sleep(time.Second * 10)
}
