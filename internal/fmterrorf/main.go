package main

import (
	"errors"
	"log/slog"
)

type SuperError struct {
	Msg string
	Err error
}

func (e *SuperError) Error() string {
	return e.Msg
}

func (e *SuperError) Unwrap() error {
	return e.Err
}

type HyperError struct {
	Msg string
	Err error
}

func (e *HyperError) Error() string {
	return e.Msg
}

func (e *HyperError) Unwrap() error {
	return e.Err
}

func main() {
	{
		var target *SuperError

		if errors.As(Super(), &target) {
			slog.Info("SuperError")
		} else {
			slog.Info("not SuperError")
		}

		if errors.As(Hyper(), &target) {
			slog.Info("SuperError")
		} else {
			slog.Info("not SuperError")
		}
	}

	{
		var target *HyperError

		if errors.As(Super(), &target) {
			slog.Info("HyperError")
		} else {
			slog.Info("not HyperError")
		}

		if errors.As(Hyper(), &target) {
			slog.Info("HyperError")
		} else {
			slog.Info("not HyperError")
		}
	}

	{
		var target *SuperError

		if errors.As(SuperHyper(), &target) {
			slog.Info("SuperError")
		} else {
			slog.Info("not SuperError")
		}

		if errors.As(HyperSuper(), &target) {
			slog.Info("SuperError")
		} else {
			slog.Info("not SuperError")
		}
	}
}

func Super() error {
	return &SuperError{
		Msg: "super",
		Err: errors.New("error"),
	}
}

func Hyper() error {
	return &HyperError{
		Msg: "hyper",
		Err: errors.New("error"),
	}
}

func SuperHyper() error {
	return &SuperError{
		Msg: "super",
		Err: &HyperError{
			Msg: "hyper",
			Err: errors.New("error"),
		},
	}
}

func HyperSuper() error {
	return &HyperError{
		Msg: "hyper",
		Err: &SuperError{
			Msg: "super",
			Err: errors.New("error"),
		},
	}
}
