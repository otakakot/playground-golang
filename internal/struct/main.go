package main

import "fmt"

func main() {
	fmt.Printf("%s\n", Hoge{})

	fmt.Printf("%s\n", Fuga{})
}

type Hoge struct{}

func (h Hoge) String() string {
	return "hoge"
}

type Fuga struct {
	*Hoge
}
