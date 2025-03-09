package actions

var _ isAbortingError = (*abortingError)(nil)

type abortingError struct {
	error
}

type isAbortingError interface {
	IsAbortingError()
}

func (e *abortingError) IsAbortingError() {}

// AbortingError marks an error as aborting, thus skipping all following actions.
func AbortingError(err error) error {
	if err == nil {
		return nil
	}
	return &abortingError{err}
}

// Unwrap implements error wrapping.
func (e *abortingError) Unwrap() error {
	return e.error
}

// Error returns the error string.
func (e *abortingError) Error() string {
	if e.error == nil {
		return "aborting: <nil>"
	}
	return "aborting: " + e.error.Error()
}

func IsAbortingError(err error) bool {
	_, ok := err.(isAbortingError)
	return ok
}
