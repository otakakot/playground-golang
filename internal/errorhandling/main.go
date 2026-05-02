package main

import "fmt"

func main() {
	err := NewCustomError(fmt.Errorf("custom error"))
	fmt.Printf("%T\n", err)
	fmt.Printf("%T\n", err.Error)
}

type CustomError struct {
	Error error
}

func NewCustomError(err error) *CustomError {
	return &CustomError{Error: err}
}
