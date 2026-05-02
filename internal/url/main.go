package main

import (
	"fmt"
	"log/slog"
	"net/url"
)

func main() {
	// Parse
	u1, err := url.Parse("https://example.com?a=b")
	if err != nil {
		panic(err)
	}

	// Query
	query := u1.Query()
	query.Set("q", "golang")
	u1.RawQuery = query.Encode()

	slog.Info(fmt.Sprintf("%+v", u1.RawQuery))

	u2, err := url.Parse("http://example.com")
	if err != nil {
		panic(err)
	}

	u2.RawQuery = query.Encode()

	slog.Info(fmt.Sprintf("%+v", u2))
}
