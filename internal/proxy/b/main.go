package main

import (
	"log/slog"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://localhost:9999/golang", http.StatusFound)
	})

	http.HandleFunc("/golang", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, Golang!"))
	})

	slog.Info("server started at http://localhost:9999")

	http.ListenAndServe(":9999", nil)
}
