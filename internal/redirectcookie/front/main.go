package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func main() {
	http.HandleFunc("/entry", func(w http.ResponseWriter, r *http.Request) {
		slog.Info(r.URL.String())

		cookie, err := r.Cookie("session")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		req, err := http.NewRequest(http.MethodGet, "http://localhost:8080/end", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		req.AddCookie(cookie)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		slog.Info(fmt.Sprintf("body: %s", string(body)))

		slog.Info(fmt.Sprintf("cookie: %s", cookie.Value))

		slog.Info(fmt.Sprintf("query: %s", r.FormValue("session")))
	})

	slog.Info("Listening on :5050")

	http.ListenAndServe(":5050", nil)
}
