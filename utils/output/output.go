// Package output writes messages for the person running gho-static.
package output

import (
	"fmt"
	"io"
	"os"
)

// Terminal escape codes to start grey, red, warning (yellow on black) and
// link (green on black) text, and return to normal text.
const (
	Grey    = "\033[90m"
	Red     = "\033[31m"
	Warning = "\033[33;40m"
	Link    = "\033[32;40m"
	Reset   = "\033[0m"
)

// Output is where messages are written. The zero value writes to stdout
// and stderr.
type Output struct {
	Out io.Writer // everything but warnings and errors
	Err io.Writer // warnings and errors

	// Quiet leaves out everything but warnings, errors and success messages.
	Quiet bool
}

// out returns where o writes everything but warnings and errors.
func (o Output) out() io.Writer {
	if o.Out == nil {
		return os.Stdout
	}
	return o.Out
}

// err returns where o writes warnings and errors.
func (o Output) err() io.Writer {
	if o.Err == nil {
		return os.Stderr
	}
	return o.Err
}

// Infof writes a message, formatted as with fmt.Printf, unless o is quiet.
func (o Output) Infof(format string, args ...any) {
	if o.Quiet {
		return
	}
	fmt.Fprintf(o.out(), format, args...)
}

// Successf writes a message saying something was done, formatted as with
// fmt.Printf, even if o is quiet.
func (o Output) Successf(format string, args ...any) {
	fmt.Fprintf(o.out(), format, args...)
}

// Warnf writes a warning line, formatted as with fmt.Printf.
func (o Output) Warnf(format string, args ...any) {
	fmt.Fprintf(o.err(), Warning+"warning: "+format+Reset+"\n", args...)
}

// Errorf writes an error line, formatted as with fmt.Printf, for errors
// that don't stop what's being done.
func (o Output) Errorf(format string, args ...any) {
	fmt.Fprintf(o.err(), "error: "+format+"\n", args...)
}

// Step writes message, runs task, then ends the line with "Done! :)" if it
// succeeded. Only plain text and newlines are written (no cursor movement),
// so it reads the same in Windows and Unix-like terminals. If o is quiet,
// it only runs task.
func (o Output) Step(message string, task func() error) error {
	if o.Quiet {
		return task()
	}
	fmt.Fprint(o.out(), message+" ")
	if err := task(); err != nil {
		// Leave the line, so the error starts on its own.
		fmt.Fprintln(o.out())
		return err
	}
	fmt.Fprintln(o.out(), "Done! :)")
	return nil
}
