package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/codelif/katnip"
	"golang.org/x/term"
)

const (
	esc = "\x1b"

	// Enable sequences
	// 1006 = SGR mouse, 1003 = any-motion (more verbose than 1002)
	// 1004 = focus in/out reporting
	enableSGR      = esc + "[?1006h"
	enableAnyMove  = esc + "[?1003h"
	enableFocusRpt = esc + "[?1004h"

	// disable sequences (always restore)
	disableSGR      = esc + "[?1006l"
	disableAnyMove  = esc + "[?1003l"
	disableFocusRpt = esc + "[?1004l"
)

var (
	sgrRe      = regexp.MustCompile(`\x1b\[\<(\d+);(\d+);(\d+)([Mm])`)
	focusInRe  = regexp.MustCompile(`\x1b\[I`)
	focusOutRe = regexp.MustCompile(`\x1b\[O`)
)

type MouseEvent struct {
	X, Y      int
	Button    string
	Action    string // press, release, move, wheel
	Modifiers []string
	RawB      int
}

// decode SGR "b" field.
// spec bits (xterm/kitty):
// - low 2 bits: 0=left,1=middle,2=right,3=release
// - +32 => motion flag (drag/move)
// - +64 => wheel (64 up, 65 down)
// - +4 shift, +8 alt, +16 ctrl
func decodeSGR(b, x, y int, final byte) MouseEvent {
	ev := MouseEvent{X: x, Y: y, RawB: b}

	if b&4 != 0 {
		ev.Modifiers = append(ev.Modifiers, "Shift")
	}
	if b&8 != 0 {
		ev.Modifiers = append(ev.Modifiers, "Alt")
	}
	if b&16 != 0 {
		ev.Modifiers = append(ev.Modifiers, "Ctrl")
	}

	isMotion := (b & 32) != 0
	isWheel := (b & 64) != 0

	switch {
	case isWheel:
		ev.Action = "wheel"
		// 64 = wheel up, 65 = wheel down (with possible +modifiers)
		switch b &^ (4 | 8 | 16) { // strip modifiers
		case 64:
			ev.Button = "WheelUp"
		case 65:
			ev.Button = "WheelDown"
		default:
			ev.Button = "Wheel?"
		}
	default:
		btn := b & 3
		switch btn {
		case 0:
			ev.Button = "Left"
		case 1:
			ev.Button = "Middle"
		case 2:
			ev.Button = "Right"
		case 3:
			ev.Button = "None"
		}

		// SGR final char: 'M' = press/drag, 'm' = release
		if isMotion && final == 'M' && ev.Button != "None" {
			ev.Action = "drag"
		} else if isMotion && ev.Button == "None" {
			ev.Action = "move"
		} else if final == 'M' {
			ev.Action = "press"
		} else {
			ev.Action = "release"
		}
	}

	return ev
}

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
	if _, err := os.Stat("./appLogs.txt"); err == nil {
		os.Remove("./appLogs.txt")
	}
	appLog, err := os.OpenFile("appLogs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		log.Fatal(err)
	}
	defer appLog.Close()
	log.SetOutput(appLog)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Println("START: panel init")

	log.Println("START: panel init")
	// switch tty to raw mode
	oldState, err := term.MakeRaw(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to enter raw mode: %v\n", err)
		return 0
	}
	defer term.Restore(int(os.Stdout.Fd()), oldState)

	// ensure cleanup on signals
	cleanup := func() {
		os.Stdout.WriteString(disableAnyMove + disableSGR + disableFocusRpt)
		os.Stdout.Sync()
	}
	defer cleanup()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sig
		cleanup()
		os.Exit(0)
	}()

	// enable terminal reports
	os.Stdout.WriteString(enableSGR + enableAnyMove + enableFocusRpt)
	os.Stdout.Sync()

	fmt.Println("raw mouse reader: SGR + any-motion + focus reporting enabled")
	fmt.Println("press 'q' to quit.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	var buf bytes.Buffer

	// optional inactivity timer example (helpful if you want to infer "left?")
	// not a real leave event; just no-activity heuristic
	last := time.Now()
	go func() {
		for range time.Tick(1500 * time.Millisecond) {
			if time.Since(last) > 2*time.Second {
				log.Println("[idle] no mouse/keys for 2s")
				last = time.Now() // throttle messages
			}
		}
	}()

	for {
		// non-blocking-ish read loop: read whatever is ready
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		last = time.Now()

		// quit on plain 'q'
		if b == 'q' {
			break
		}

		buf.WriteByte(b)

		// try to peel off complete SGR mouse and focus tokens
		data := buf.Bytes()

		// focus in/out
		for {
			loc := focusInRe.FindIndex(data)
			if loc == nil {
				break
			}
			log.Println("FOCUS-IN")
			// drop that slice
			data = append(data[:loc[0]], data[loc[1]:]...)
		}
		for {
			loc := focusOutRe.FindIndex(data)
			if loc == nil {
				break
			}
			log.Println("FOCUS-OUT")
			data = append(data[:loc[0]], data[loc[1]:]...)
		}

		// SGR mouse (can be multiple packed together)
		for {
			m := sgrRe.FindSubmatchIndex(data)
			if m == nil {
				break
			}
			// extract groups
			// groups: 1=b, 2=x, 3=y, 4=final
			bStr := data[m[2]:m[3]]
			xStr := data[m[4]:m[5]]
			yStr := data[m[6]:m[7]]
			final := data[m[8]:m[9]][0]

			var bb, xx, yy int
			fmt.Sscanf(string(bStr), "%d", &bb)
			fmt.Sscanf(string(xStr), "%d", &xx)
			fmt.Sscanf(string(yStr), "%d", &yy)

			ev := decodeSGR(bb, xx, yy, final)
			log.Printf("MOUSE %-7s  btn=%-8s", ev.Action, ev.Button)

			// remove the matched slice and continue scanning the rest
			data = append(data[:m[0]], data[m[1]:]...)
		}

		// keep any tail bytes that didn't match a full token (partial escape)
		buf.Reset()
		buf.Write(data)
	}

	// restore on exit
	cleanup()

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
		FocusPolicy: katnip.FocusExclusive,
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
