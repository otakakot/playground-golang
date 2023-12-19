package main

import (
	"log/slog"
	"net/http"

	"github.com/vmihailenco/msgpack/v5"
)

func main() {
	http.HandleFunc("/message", Handle)

	slog.Info("Server is running on http://localhost:8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

type Message struct {
	ID   string
	Byte []byte
}

func Handle(w http.ResponseWriter, r *http.Request) {
	msg := Message{
		ID:   "1",
		Byte: []byte("hello"),
	}

	res, err := msgpack.Marshal(msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	w.Header().Set("Content-Type", "application/x-msgpack")
	w.WriteHeader(http.StatusOK)
	w.Write(res)
}
