package main

import (
	"fmt"
	"log/slog"
)

type Hoge struct {
	ID   string
	Name string
}

func (h *Hoge) SetID(id string) {
	h.ID = id
}

func (h Hoge) SetID2(id string) Hoge {
	return Hoge{
		ID:   id,
		Name: h.Name,
	}
}

func (h Hoge) String() (Hoge, error) {
	return Hoge{}, fmt.Errorf("")
}

func main() {

	hoge := Hoge{
		ID: "1",
	}

	hoge.SetID("2")

	slog.Info(hoge.ID) //2

	hoge.SetID2("3")

	slog.Info(hoge.ID) //3
}
