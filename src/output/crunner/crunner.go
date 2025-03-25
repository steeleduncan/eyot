package crunner

import (
	"io"
)

// A platform agnostic interface for running a C Compiler
type CRunner interface {
	// "Open" the stream for the runner, writing to the given path
	Open(path string) error

	// The stream to write the C to
	WriteStream() io.Writer

	/*
	  "Close" the stream for the runner, and guaranteeing it has tried to write C. Return the log on failure
	  When compiling, if the withOpenCl flag is true, it'll be
	*/
	Close(compile bool, withOpenCl bool, defs map[string]string, supportFiles []string, flags map[string]bool) (string, error)
}
