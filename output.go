package anansi

import (
	"errors"
	"io"
	"os"
	"syscall"
	"time"
)

// NewOutput creates a new terminal output writer around the given file; if
// using Output Context-ually, the user must pass nil to the constructor,
// allowing Enter() to set it.
func NewOutput(f *os.File) *Output {
	return &Output{file: f}
}

// Output supports writing buffered output from a io.WriterTo (implemented by
// both Cursor and Screen) into a file handle (presumably attached to a
// terminal). It is not safe to use Output in parallel from multiple
// goroutines, such users need to layer a lock around an Output.
type Output struct {
	Flushed int
	blocks  []time.Duration
	file    *os.File
}

// TrackStalls allocates a buffer for tracking stall times; otherwise Stalls()
// will always return nil. If output is used with a non-blocking file handle,
// and if a Flush() write encounters syscall.EWOULDBLOCK, then it switches the
// file handle into blocking mode, and performs a blocking write.
//
// Such blocking flush is counted as a "stall", and the total time spent
// performing FCNTL syscalls and the blocking is counted in any internal buffer
// (allocated by TrackStalls) for further collection and reporting. Once the
// buffer fills, such timing measurements cease to be taken, as if no buffer
// was available. Users interested in collecting these metrics should attempt
// to harvest data using the Stalls() method, and only process such data when
// it is full (len() == cap()).
func (out *Output) TrackStalls(n int) {
	if n == 0 {
		out.blocks = nil
	} else {
		out.blocks = make([]time.Duration, 0, n)
	}
}

// Stalls resets the stalls counter, returning the prior value; a stall happens
// when a flush must do a blocking write on an otherwise non-blocking
// underlying file. The caller must use the returned duration slice
// immediately, as it will be reused if full or if consume was true.
func (out *Output) Stalls(consume bool) []time.Duration {
	if out.blocks == nil {
		return nil
	}
	blocks := out.blocks
	if len(blocks) == cap(blocks) || consume {
		out.blocks = blocks[:0]
	}
	return blocks
}

// Enter retains the passed file handle, enabling Flush().
// Panics if another file handle has already been set.
func (out *Output) Enter(term *Term) error {
	if out.file != nil {
		return errors.New("anansi.Output may only only be attached to one terminal")
	}
	out.file = term.File
	return nil
}

// Exit clears the retained file handle, disabling Flush().
// Panics if the retained file handle is different from the one passed.
func (out *Output) Exit(term *Term) error {
	if out.file == term.File {
		out.file = nil
	}
	return nil
}

// Flush calls the given io.Writerto on any active file handle. If EWOULDBLOCK
// occurs, it transitions the file into blocking mode, and restarts the write.
func (out *Output) Flush(wer io.WriterTo) error {
	if out.file == nil {
		return nil
	}
	out.Flushed = 0
	n, err := wer.WriteTo(out.file)
	out.Flushed += int(n)
	if isEWouldBlock(err) {
		return out.blockingFlush(wer)
	}
	return err
}

func (out *Output) blockingFlush(wer io.WriterTo) error {
	if out.blocks != nil {
		defer func(t0 time.Time) {
			t1 := time.Now()
			if len(out.blocks) < cap(out.blocks) {
				out.blocks = append(out.blocks, t1.Sub(t0))
			}
		}(time.Now())
	}

	const mask = syscall.O_NONBLOCK | syscall.O_ASYNC

	flags, _, e := syscall.Syscall(syscall.SYS_FCNTL, out.file.Fd(), syscall.F_GETFL, 0)
	if e != 0 {
		return e
	}

	if _, _, e = syscall.Syscall(syscall.SYS_FCNTL, out.file.Fd(), syscall.F_SETFL, 0); e != 0 {
		return e
	}

	n, err := wer.WriteTo(out.file)
	out.Flushed += int(n)

	if _, _, e = syscall.Syscall(syscall.SYS_FCNTL, out.file.Fd(), syscall.F_SETFL, flags&mask); e != 0 {
		if err == nil {
			err = e
		}
	}

	return err
}
