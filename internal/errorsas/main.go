package main

import (
	"errors"
	"net/http"
)

func main() {
	if err := http.ListenAndServe(":8080", nil); err != nil && errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

var ErrNotFound = errors.New("not found")
