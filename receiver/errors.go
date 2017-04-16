package receiver

type ExposableError struct {
	err string
}

func NewExposableError(err string) ExposableError {
	return ExposableError{
		err: err,
	}
}

func (e ExposableError) Error() string {
	return e.err
}
