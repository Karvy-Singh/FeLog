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
	"syscall"
	"time"

	"github.com/Karvy-Singh/FeLog/internals/mouse"
	"github.com/codelif/katnip"
	"golang.org/x/term"
)

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
	out := "/tmp/kitty_panel_rounded.png"
	width := 300
	height := 300

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

	// switch tty to raw mode
	oldState, err := term.MakeRaw(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to enter raw mode: %v\n", err)
		return 0
	}
	defer term.Restore(int(os.Stdout.Fd()), oldState)

	cleanup := func() {
		os.Stdout.WriteString(mouse.DisableAnyMove + mouse.DisableSGR + mouse.DisableFocus + mouse.DisableSGRPixels)
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
	os.Stdout.WriteString(mouse.EnableSGR + mouse.EnableAnyMove + mouse.EnableFocus + mouse.EnableSGRPixels)
	os.Stdout.Sync()

	fmt.Println("raw mouse reader: SGR + any-motion + focus reporting enabled")
	fmt.Println("press 'q' to quit.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	var buf bytes.Buffer

	// for some reason this is here, will remove later, but MIGHT prove to be useful in some cases.
	last := time.Now()
	go func() {
		for range time.Tick(1500 * time.Millisecond) {
			if time.Since(last) > 2*time.Second {
				log.Println("[idle] no mouse/keys for 2s")
				last = time.Now()
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

		buf.WriteByte(b)

		// try to peel off complete SGR mouse and focus tokens
		data := buf.Bytes()

		data, focusIn, focusOut, mouseEV := mouse.ExtractEvents(data)
		if focusIn != 0 {
			log.Printf("FOCUS-IN")
		}
		if focusOut != 0 {
			log.Printf("FOCUS-OUT")
		}
		if mouseEV.Motion != "" || mouseEV.Click != "" {
			log.Printf("MOUSE %-7s  btn=%-8s", mouseEV.Motion, mouseEV.Click)
		}

		// focus in/out
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
	width, height := 300, 300
	radius := 24
	color := "rgba(30,30,46,1)"
	out := "/tmp/kitty_panel_rounded.png"

	err := makeRoundedPNG(out, width, height, radius, color)
	if err != nil {
		log.Fatal(err)
	}

	cfg := katnip.Config{
		Edge:        katnip.EdgeNone,
		Position:    katnip.Vector{X: 300, Y: 300},
		Layer:       katnip.LayerTop,
		FocusPolicy: katnip.FocusOnDemand,
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
