package main

import (
	"errors"
	"flag"
	"image"
	"io"
	"log"
	"os"
	"strings"

	"github.com/jcorbin/anansi"
	"github.com/jcorbin/anansi/ansi"
	"github.com/jcorbin/anansi/x/platform"
)

var errInt = errors.New("interrupt")

func main() {
	platform.MustRun(os.Stdout, Run, platform.FrameRate(60))
}

// Run the demo under an active terminal platform.
func Run(p *platform.Platform) error {

	cmd := flag.Args()

	for {
		in := inspect{}
		in.setCmd(cmd)
		if err := p.Run(&in); platform.IsReplayDone(err) {
			continue // loop replay
		} else if err == io.EOF || err == errInt {
			return nil
		} else if err != nil {
			log.Printf("exiting due to %v", err)
			return err
		}
	}
}

type inspect struct {
	cmd  []string
	argi []int
	arg  []string

	ed platform.EditLine

	cmdOutput anansi.Grid
}

func (in *inspect) setCmd(cmd []string) {
	in.cmd = append(in.cmd[:0], cmd...)
	in.argi = in.argi[:0]
	in.arg = in.arg[:0]
	for i, arg := range in.cmd {
		if strings.HasPrefix(arg, "\\$") {
			in.cmd[i] = arg[1:]
		} else if strings.HasPrefix(arg, "$") {
			in.arg = append(in.arg, arg[1:])
			in.argi = append(in.argi, i)
		}
	}
}

func (in *inspect) Update(ctx *platform.Context) (err error) {
	// Ctrl-C interrupts
	if ctx.Input.HasTerminal('\x03') {
		// ... AFTER any other available input has been processed
		err = errInt
		// ... NOTE err != nil will prevent wasting any time flushing the final
		//          lame-duck frame
	}

	// Ctrl-Z suspends
	if ctx.Input.CountRune('\x1a') > 0 {
		defer func() {
			if err == nil {
				err = ctx.Suspend()
			} // else NOTE don't bother suspending, e.g. if Ctrl-C was also present
		}()
	}

	ctx.Output.Clear()
	p := image.Pt(1, 1)
	if ctx.HUD.Visible {
		p.Y++
	}
	ctx.Output.To(p)

	j := 0
	for i, arg := range in.cmd {
		if i > 0 {
			ctx.Output.WriteRune(' ')
		}
		var attr ansi.SGRAttr
		if j < len(in.argi) && in.argi[j] == i {
			attr = ansi.SGRCyan.FG()
			// TODO interaction
		} else if i == 0 {
			attr = ansi.SGRGreen.FG()
		}
		if attr != 0 {
			ctx.Output.WriteSGR(attr)
			ctx.Output.WriteString(arg)
			ctx.Output.WriteSGR(ansi.SGRAttrClear)
		} else {
			ctx.Output.WriteString(arg)
		}
	}

	// TODO scroll w/in cmdOutput

	return err
}
