package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

const name = "session"

var mem = make(map[string]string)

func main() {
	http.HandleFunc("/begin", func(w http.ResponseWriter, r *http.Request) {
		slog.Info(r.URL.String())

		session := uuid.NewString()

		mem[session] = uuid.NewString()

		cookie := http.Cookie{
			Name:     name,
			Value:    session,
			Expires:  time.Now().Add(time.Hour),
			Secure:   true,
			HttpOnly: true,
		}

		slog.Info(fmt.Sprintf("cookie: %s", cookie.Value))

		w.Header().Set("Set-Cookie", cookie.String())

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Add("Access-Control-Allow-Headers", "*")

		redirect, err := url.Parse("http://localhost:5050/entry?session=" + cookie.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, redirect.String(), http.StatusFound)
	})

	http.HandleFunc("/end", func(w http.ResponseWriter, r *http.Request) {
		slog.Info(r.URL.String())

		cookie, err := r.Cookie(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		slog.Info(fmt.Sprintf("cookie: %s", cookie.Value))

		val := mem[cookie.Value]

		delete(mem, cookie.Value)

		w.Write([]byte(val))
	})

	slog.Info("Listening on :8080")

	http.ListenAndServe(":8080", nil)
}
