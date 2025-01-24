package actions

import "fmt"

var ErrAborted = &abortingError{err: fmt.Errorf("aborted")}

type abortingError struct {
	err error
}

// AbortingError marks an error as aborting, thus skipping all following actions.
func AbortingError(err error) error {
	if err == nil {
		return nil
	}
	return &abortingError{err}
}

// Unwrap implements error wrapping.
func (e *abortingError) Unwrap() error {
	return e.err
}

// Error returns the error string.
func (e *abortingError) Error() string {
	if e.err == nil {
		return "aborting: <nil>"
	}
	return "aborting: " + e.err.Error()
}
