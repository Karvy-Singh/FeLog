package tui

import (
	"fmt"
	"os"
	"os/exec"
)

func MakeRoundedPNG(out string, width, height, radius int, rgba string) error {
	cmd := exec.Command(
		"magick",
		"-size", fmt.Sprintf("%dx%d", width, height), "canvas:none",
		"-fill", rgba,
		"-draw", fmt.Sprintf("roundrectangle 0,0 %d,%d %d,%d", width-1, height-1, radius, radius),
		"PNG32:"+out,
	)
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Run()
}

func RenderIcat(out string, cols, lines string, width, height int) error {
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
	icat.Stdout, icat.Stderr = os.Stdout, nil
	return icat.Run()
}
