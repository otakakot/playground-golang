package main

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url, err := url.Parse("http://localhost:9999")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.ServeHTTP(w, r)
	})

	slog.Info("server started at http://localhost:8888")

	http.ListenAndServe(":8888", nil)
}
