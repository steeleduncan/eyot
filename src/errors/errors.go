package errors

import (
	"fmt"
	"io"
)

type SourceLocation struct {
	Filename string
	Line     int
}

func (sl *SourceLocation) String() string {
	if sl.Line < 0 {
		return fmt.Sprintf("%v: EOF", sl.Filename)
	} else {
		return fmt.Sprintf("%v: %v", sl.Filename, sl.Line)
	}
}

type ErrorMessage struct {
	Location SourceLocation
	Message  string
}

func (em *ErrorMessage) String() string {
	return fmt.Sprintf("%v: %v", em.Location.String(), em.Message)
}

type Errors struct {
	errorMessages     []ErrorMessage
	lastKnownLocation SourceLocation
	internalError     error
}

func NewErrors() *Errors {
	return &Errors{
		errorMessages: []ErrorMessage{},
		internalError: nil,
	}
}

// return any internal error
func (es *Errors) InternalError() error {
	return es.internalError
}

// Log an error with exec
func (es *Errors) LogInternalError(err error) {
	es.internalError = err
}

func (es *Errors) SetCurrentLocation(sl SourceLocation) {
	es.lastKnownLocation = sl
}

func (es *Errors) Errorf(format string, args ...interface{}) {
	em := ErrorMessage{
		Location: es.lastKnownLocation,
		Message:  fmt.Sprintf(format, args...),
	}
	es.errorMessages = append(es.errorMessages, em)
}

func (es *Errors) Clean() bool {
	if es.internalError != nil {
		return false
	}

	return len(es.errorMessages) == 0
}

// print human readable errors
//
// return true if no errors, false otherwise
func (es *Errors) LogErrors(w io.Writer) bool {
	if len(es.errorMessages) == 0 {
		return true
	}

	for _, emsg := range es.errorMessages {
		fmt.Fprintln(w, emsg.String())
	}

	return false
}
