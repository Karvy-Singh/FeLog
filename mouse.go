package main

import (
	"regexp"
)

// referred from:
// https://invisible-island.net/xterm/ctlseqs/ctlseqs.pdf
// PAGE-15-19
// found it better than other docs...

const (
	esc = "\x1b"

	EnableSGR       = esc + "[?1006h"
	EnableAnyMove   = esc + "[?1003h" // report all motion
	EnableFocus     = esc + "[?1004h" // focus in/out
	EnableSGRPixels = esc + "[?1016h" // sgr pixels, potentially lead to kitty mouse left window event check

	DisableSGR       = esc + "[?1006l"
	DisableAnyMove   = esc + "[?1003l"
	DisableFocus     = esc + "[?1004l"
	DisableSGRPixels = esc + "[?1016l"
)

var (
	sgrRe      = regexp.MustCompile(`\x1b\[\<(\d+);(\d+);(\d+)([Mm])`)
	focusInRe  = regexp.MustCompile(`\x1b\[I`)
	focusOutRe = regexp.MustCompile(`\x1b\[O`)
)

type MouseEvent struct {
	X, Y      int
	ButtonVal int
	Click     string // left, right, middle etc.
	Motion    string // press, release, move etc.
}

// ref for below: pg-51; https://invisible-island.net/xterm/ctlseqs/ctlseqs.pdf

// SGR (1006) The normal mouse response is altered to use
// • CSI < followed by semicolon-separated
// • encoded button value,
// • Px and Py ordinates and
// • a final character which is M for button press and m for button release.

func decodeSGR(b, x, y int, final byte) MouseEvent {
	ev := MouseEvent{X: x, Y: y, ButtonVal: b}
	isMotion := (b & 32) != 0

	switch {
	default:
		btn := b & 3
		switch btn {
		case 0:
			ev.Click = "Left"
		case 1:
			ev.Click = "Middle"
		case 2:
			ev.Click = "Right"
		case 3:
			ev.Click = "None"
		}

		// final char: M-press, m-release
		if isMotion && final == 'M' && ev.Click != "None" {
			ev.Motion = "drag"
		} else if isMotion && ev.Click == "None" {
			ev.Motion = "move"
		} else if final == 'M' {
			ev.Motion = "press"
		} else {
			ev.Motion = "release"
		}
	}

	return ev
}
