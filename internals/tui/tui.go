package tui

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codelif/katnip"
	"golang.org/x/term"

	"github.com/Karvy-Singh/FeLog/internals/mouse"
	"github.com/Karvy-Singh/FeLog/internals/utils"
)

type Config struct {
	Width, Height  int
	X, Y           int
	Radius         int
	ColorRGBA      string
	Layer          katnip.Layer
	FocusPolicy    katnip.FocusPolicy
	KittyOverrides []string
}

type Option func(*Config)

func WithSize() Option         { return func(c *Config) { c.Width, c.Height = 300, 300 } }
func WithPosition() Option     { return func(c *Config) { c.X, c.Y = 300, 300 } }
func WithCornerRadius() Option { return func(c *Config) { c.Radius = 24 } }
func WithPanelColor() Option   { return func(c *Config) { c.ColorRGBA = "rgba(30,30,46,1)" } }
func WithLayer() Option        { return func(c *Config) { c.Layer = katnip.LayerTop } }
func WithFocusPolicy() Option {
	return func(c *Config) { c.FocusPolicy = katnip.FocusOnDemand }
}

func WithKittyOverrides(k ...string) Option {
	return func(c *Config) { c.KittyOverrides = append(c.KittyOverrides, k...) }
}

type TUI struct {
	cfg     Config
	imgPath string
}

func New(opts ...Option) (*TUI, error) {
	c := Config{
		Width:       300,
		Height:      300,
		Radius:      16,
		ColorRGBA:   "rgba(30,30,46,1)",
		Layer:       katnip.LayerTop,
		FocusPolicy: katnip.FocusOnDemand,
	}

	for _, o := range opts {
		o(&c)
	}

	return &TUI{cfg: c, imgPath: "/tmp/kitty_panel_rounded.png"}, nil
}

func (t *TUI) Run() int {
	infoLog, err := utils.MakeLogger("appLogs")
	if err != nil {
		log.Fatal(err)
	}

	errorLog, err := utils.MakeLogger("errorLog")
	if err != nil {
		log.Fatal(err)
	}

	infoLog.Println("START: tui init")

	if err := MakeRoundedPNG(t.imgPath, t.cfg.Width, t.cfg.Height, t.cfg.Radius, t.cfg.ColorRGBA); err != nil {
		errorLog.Println("rounded png:", err)
		return 1
	}
	defer os.Remove(t.imgPath)

	fd := int(os.Stdout.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		errorLog.Println(os.Stderr, "failed to enter raw mode: %v\n", err)
		return 1
	}
	defer term.Restore(fd, oldState)

	cleanup := func() {
		os.Stdout.WriteString("\033[?25h" + mouse.DisableAnyMove + mouse.DisableSGR + mouse.DisableFocus + mouse.DisableSGRPixels)
		os.Stdout.Sync()
	}
	defer cleanup()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	go func() { <-ctx.Done(); cleanup(); os.Exit(0) }()

	os.Stdout.WriteString("\033[?25l" + mouse.EnableSGR + mouse.EnableAnyMove + mouse.EnableFocus + mouse.EnableSGRPixels)
	os.Stdout.Sync()

	fmt.Println("raw mouse reader: SGR + any-motion + focus reporting enabled")
	fmt.Println("press 'q' to quit.\n")

	if err := RenderIcat(t.imgPath, t.cfg.Width, t.cfg.Height); err != nil {
		errorLog.Println("icat:", err)
		return 1
	}

	// input loop
	reader := bufio.NewReader(os.Stdin)
	var buf bytes.Buffer
	last := time.Now()

	// for some reason this is here, will remove later, but MIGHT prove to be useful in some cases.
	go func() {
		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if time.Since(last) > 2*time.Second {
				infoLog.Println("[idle] no mouse/keys for 2s")
				last = time.Now()
			}
		}
	}()

	for {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		if b == 'q' || b == 'Q' {
			break
		}
		last = time.Now()
		buf.WriteByte(b)

		data := buf.Bytes()
		data, focusIn, focusOut, ev := mouse.ExtractEvents(data)
		if focusIn != 0 {
			infoLog.Printf("FOCUS-IN")
		}
		if focusOut != 0 {
			infoLog.Printf("FOCUS-OUT")
		}
		if ev.Motion != "" || ev.Click != "" {
			infoLog.Printf("MOUSE %-7s  btn=%-8s", ev.Motion, ev.Click)
		}

		buf.Reset()
		buf.Write(data)
	}

	cleanup()
	return 0
}
