// Package cmdutil provides shared CLI utilities including IOStreams and Factory.
package cmdutil

import (
	"io"
	"os"
)

// IOStreams abstracts standard input, output, and error streams.
type IOStreams struct {
	In     io.ReadCloser
	Out    io.Writer
	ErrOut io.Writer
}

// DefaultIOStreams returns IOStreams wired to os.Stdin, os.Stdout, os.Stderr.
func DefaultIOStreams() *IOStreams {
	return &IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
}
