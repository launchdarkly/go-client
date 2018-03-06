package ferror

type WellformednessError struct {
	Message string
}

func (err WellformednessError) Error() string {
	return err.Message
}

func NewWellformednessError(message string) WellformednessError {
	return WellformednessError{message}
}

func IsWellformednessError(err error) bool {
	switch err.(type) {
	case WellformednessError:
		return true
	}
	return false
}
