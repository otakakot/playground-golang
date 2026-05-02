package main

import (
	"fmt"

	"github.com/otakakot/playground-golang/internal/enum/color"
	"github.com/otakakot/playground-golang/internal/enum/elegant"
)

func main() {
	PrintColor(color.Blue)
	PrintColor("rainbow")

	PrintElegant(elegant.Red)
	// PrintElegant("orange")
}

func PrintColor(
	c color.Color,
) {
	fmt.Println(c)
}

func PrintElegant(
	c elegant.Color,
) {
	fmt.Println(c)
}

type mycolor string

func (c mycolor) protected() {}

const black mycolor = "black"

func (c mycolor) String() string {
	return string(c)
}
