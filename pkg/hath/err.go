package hath

var _ error = (*HTTPErr)(nil)

// HTTPErr contains http status code
type HTTPErr struct {
	Status int
	Err    error
}

func (err *HTTPErr) Error() string {
	return err.Err.Error()
}

// NewHTTPErr ...
func NewHTTPErr(status int, err error) *HTTPErr {
	return &HTTPErr{
		Status: status,
		Err:    err,
	}
}
