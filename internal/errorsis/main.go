package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/lib/pq"
)

func main() {

}

func Handle(w http.ResponseWriter, r *http.Request) {
	client, _ := sql.Open("postgres", "dsn")

	if _, err := client.ExecContext(context.Background(), "SELECT 1"); err != nil {
		w.WriteHeader(ToHTTPStatus(err))

		return
	}

	w.WriteHeader(http.StatusOK)
}

func ToHTTPStatus(err error) int {
	var target *pq.Error

	if errors.As(err, &target) {
		switch target.SQLState() {
		case "23505":
			return http.StatusConflict
		// ...
		default:
			return http.StatusInternalServerError
		}
	}

	return http.StatusInternalServerError
}
