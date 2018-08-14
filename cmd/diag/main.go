package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"unicode/utf8"

	"github.com/jcorbin/anansi"
	"github.com/jcorbin/anansi/ansi"
)

var errInt = errors.New("interrupt")

var (
	logFile      = flag.String("log", "", "debug log file (default stderr)")
	doReset      = flag.Bool("reset", false, "enable terminal resetting")
	noRaw        = flag.Bool("no-raw", false, "disable raw mode")
	useAltScreen = flag.Bool("alt-screen", false, "enable alternate screen mode")
	useMouse     = flag.Bool("mouse", false, "enable mouse reporting")
)

var csiCUP = ansi.CSI('H')

func main() {
	flag.Parse()

	if *logFile != "" {
		f, err := os.Create(*logFile)
		if err != nil {
			log.Fatalf("couldn't open logFile %q: %v", *logFile, err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	input := anansi.NewInput(os.Stdin, 0)
	term := anansi.NewTerm(os.Stdout)
	term.SetTermReset(*doReset)
	term.SetSGRReset(*doReset)
	term.SetRaw(!*noRaw)
	term.SetAlternateScreen(*useAltScreen)
	term.SetMouseReporting(*useMouse)

	switch err := term.With(func(term *anansi.Term) error {
		d := diag{
			Term:  term,
			Input: input,
		}
		return d.run()
	}); err {
	case nil:
	case io.EOF, errInt:
		fmt.Println(err)
	default:
		log.Fatal(err)
	}
}

type diag struct {
	*anansi.Term
	*anansi.Input

	output outputBuffer

	size  image.Point
	mouse struct {
		e ansi.Escape
		a string
	}
}

func (d *diag) run() error {
	for {
		n, err := d.Input.ReadMore()
		if n > 0 {
			if perr := d.process(); err == nil {
				err = perr
			}
		}
		if err != nil {
			return err
		}
	}
}

func (d *diag) process() error {
	d.size, _ = d.Term.Size() // TODO SIGWINCH

	for {
		var err error
		if e, a := d.Input.DecodeEscape(); e != 0 {
			err = d.handleInput(e, a, 0)
		} else if r, ok := d.Input.DecodeRune(false); ok {
			err = d.handleInput(0, nil, r)
		} else {
			break
		}
		if err != nil {
			return err
		}
	}

	return d.output.do(d.Term.File, func() {
		d.updateStatus()
	})
}

func (d *diag) handleInput(e ansi.Escape, a []byte, r rune) error {
	switch e {
	case 0:
		break // noop on null byte

	case ansi.CSI('M'), ansi.CSI('m'):
		d.mouse.e, d.mouse.a = e, string(a)
		return nil

	default:
		fmt.Print(e)
		if len(a) > 0 {
			fmt.Printf(" %q", a)
		}
		fmt.Printf("\r\n")
		return nil
	}

	switch {
	// advance line on <Enter>
	case r == '\x0d':
		fmt.Printf("\r\n")

	// simulate EOF on Ctrl-D
	case r == '\x04':
		fmt.Printf("^D\r\n")
		return io.EOF

	// stop on Ctrl-C
	case r == '\x03':
		fmt.Printf("^C\r\n")
		return errInt

	// suspend on Ctrl-Z
	case r == '\x1a':
		fmt.Printf("^Z\r\n")
		if err := d.Term.Without(anansi.Suspend); err != nil {
			return err
		}
		fmt.Printf("resumed\r\n")

	// print C0 controls phonetically
	case r < 0x20, r == 0x7f:
		fmt.Printf("^%s", string(0x40^r))

	// print C1 controls mnemonically
	case 0x80 <= r && r <= 0x9f:
		fmt.Print(ansi.C1Names[r&0x1f])

	// print normal rune
	default:
		fmt.Print(string(r))
	}

	return nil
}

func (d *diag) updateStatus() {
	d.output.withSavedCursor(func() {
		d.output.drawRightAlignedLines(d.size, []string{
			fmt.Sprintf("Size: %+ 20v", d.size),
			fmt.Sprintf("Mouse: % 20s", fmt.Sprintf("%v %q", d.mouse.e, d.mouse.a)),
		})
	})
}

var (
	saveCursor    = ansi.ESC('7')
	restoreCursor = ansi.ESC('8')
)

type outputBuffer struct {
	bytes.Buffer
}

func (out *outputBuffer) WriteSeq(seqs ...ansi.Seq) (n int) {
	for i := range seqs {
		n += seqs[i].WriteIntoBuffer(&out.Buffer)
	}
	return n
}

func (out *outputBuffer) do(w io.Writer, f func()) error {
	out.Reset()
	f()
	_, err := out.WriteTo(w)
	return err
}

func (out *outputBuffer) withSavedCursor(f func()) {
	saveCursor.WriteIntoBuffer(&out.Buffer)
	f()
	restoreCursor.WriteIntoBuffer(&out.Buffer)
}

func (out *outputBuffer) drawRightAlignedLines(at image.Point, parts []string) {
	var max int
	for i := range parts {
		n := utf8.RuneCountInString(parts[i])
		if max < n {
			max = n
		}
	}
	for i := range parts {
		n := utf8.RuneCountInString(parts[i])
		out.WriteSeq(csiCUP.WithInts(1+i, 1+at.X-n))
		_, _ = out.WriteString(parts[i])
	}
}
