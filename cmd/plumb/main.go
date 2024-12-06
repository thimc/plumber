// Command plumb implements a user-friendly command line tool to
// interface the plumber.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/thimc/plumber/internal"
)

var (
	wd, _     = os.Getwd()
	plumbfile = flag.String("p", "send", "plumb file")
	dst       = flag.String("d", "", "dst field")
	src       = flag.String("s", "plumb", "src field")
	typ       = flag.String("t", "text", "type field")
	wdir      = flag.String("w", wd, "wdir field")
	stdin     = flag.Bool("i", false, "read data standard input rather than using argument strings")
)

func main() {
	internal.Defaults()
	flag.Parse()
	f, err := os.OpenFile(*plumbfile, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	data := strings.Join(flag.Args(), " ")
	if *stdin {
		buf, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if len(buf) == 0 {
			fmt.Fprintln(os.Stderr, "no input provided")
			os.Exit(1)
		}
		data = string(buf) + "\n"
	}
	msg := internal.Message{
		Src:  *src,
		Dst:  *dst,
		Wdir: *wdir,
		Type: *typ,
		Data: []byte(data),
	}
	if err := msg.Encode(f); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
