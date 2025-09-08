package main

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/codelif/katnip"
)

// code that runs *inside* the overlay panel
func init() {
	katnip.RegisterFunc("demo", func(k *katnip.Kitty, rw io.ReadWriter) int {
		for i := 1; i <= 5; i++ {
			fmt.Printf("hello from katnip (line %d)\n", i)
			time.Sleep(1 * time.Second) // keep the panel visible
		}
		return 0
	})
}

func main() {
	// simplest: a top-edge panel 8 lines high
	p := katnip.TopPanel("demo", 8)
	if err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
