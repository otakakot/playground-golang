package main

import "errors"

type ptrError struct {
	msg string
	err error
}

func NewPtrError(msg string, err error) error {
	return &ptrError{
		msg: msg,
		err: err,
	}
}

func (err *ptrError) Error() string {
	if err.err != nil {
		return err.msg + ": " + err.err.Error()
	}

	return err.msg
}

func (err *ptrError) Unwrap() error {
	return err.err
}

func IsPtrError(err error) bool {
	var target *ptrError

	return errors.As(err, &target)
}

type valError struct {
	msg string
	err error
}

func NewValError(msg string, err error) error {
	return &valError{
		msg: msg,
		err: err,
	}
}

func (err *valError) Error() string {
	if err.err != nil {
		return err.msg + ": " + err.err.Error()
	}

	return err.msg
}

func (err *valError) Unwrap() error {
	return err.err
}

func IsValError(err error) bool {
	var target *valError

	return errors.As(err, &target)
}

func isValError(err error) bool {
	var target valError

	return errors.As(err, &target)
}

func main() {
	err1 := NewPtrError("this is a custom error", nil)

	println(IsPtrError(err1)) // true

	err2 := &ptrError{}

	println(IsPtrError(err2)) // true

	err3 := ptrError{}

	println(IsPtrError(&err3)) // true

	err4 := NewValError("this is a custom error", nil)

	println(IsValError(err4)) // true

	println(isValError(err4)) // true

	err5 := &valError{}

	println(IsValError(err5)) // true

	err6 := valError{}

	println(IsValError(&err6)) // true
}
