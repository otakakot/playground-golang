package main

import (
	"fmt"
	"log"
)

func main() {

	z1, _ := Zero()

	Save(z1) // ゼロ値が保存される

	z2, _ := Nil()

	Save(*z2) // panicが発生し処理が中断する
}

type Value struct {
	Value string
}

func Zero() (Value, error) {
	return Value{}, fmt.Errorf("error")
}

func Nil() (*Value, error) {
	return nil, fmt.Errorf("error")
}

func Save(
	val Value,
) {
	log.Printf("%+v", val.Value)
}
