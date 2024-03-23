// to improve https://go.dev/blog/error-handling-and-go

package errors

import (
	"fmt"

	"github.com/dragosh/zen/pkg/api"
)

type AppError struct {
	Err     error
	Message string
}

// var log = logger.Create()

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func Wrap(err error, info string) *AppError {
	return &AppError{
		Err:     err,
		Message: info,
	}
}

//	func Create(logger logger.Logger) *AppError {
//		return &AppError{logger: logger}
//	}
func Handle(err error) {
	if err != nil {
		api.Log.Error(err)
	}
}
