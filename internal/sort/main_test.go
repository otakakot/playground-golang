package main_test

import (
	"slices"
	"sort"
	"testing"
)

func Sort(x int) {
	ar := make([]int, x)
	for i := 0; i < x; i++ {
		ar[i] = x + i
	}

	sort.Ints(ar)
}

func Slice(x int) {
	ar := make([]int, x)
	for i := 0; i < x; i++ {
		ar[i] = x + i
	}

	slices.Sort(ar)
}

func BenchmarkSort1000(b *testing.B) {
	Sort(1000)
}

func BenchmarkSlice1000(b *testing.B) {
	Slice(1000)
}
