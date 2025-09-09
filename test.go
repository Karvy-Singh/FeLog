package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/codelif/katnip"
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

func panel(k *katnip.Kitty, rw io.ReadWriter) int {
	width, height := 800, 500
	out := "/tmp/kitty_panel_rounded.png"

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
		out,
	)
	icat.Env = os.Environ()
	icat.Stdout, icat.Stderr = os.Stdout, os.Stderr
	_ = icat.Run()

	fmt.Println("hello world")

	select {}
}

func init() {
	katnip.RegisterFunc("rounded-demo", panel)
}

func main() {
	width, height := 800, 500
	radius := 24
	color := "rgba(30,30,46,1)"
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
		},
		Overrides: map[string]string{
			"--lines":   fmt.Sprintf("%dpx", height),
			"--columns": fmt.Sprintf("%dpx", width),
		},
		// SingleInstance: true,
		// InstanceGroup:  "rounded-demo",
	}

	p := katnip.NewPanel("rounded-demo", cfg)
	p.Run()
}
