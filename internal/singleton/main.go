package main

func main() {
}

type Hoge struct{}

type Singleton[T Singleton[T]] struct {
	instance *T
}

func (s *Singleton[T]) New() *T {
	if s.instance == nil {
		s.instance = new(T)
	}
	return s.instance
}

// type Singleton[T Singleton[T]] interface {
// 	New() T
// }

// var _ Singleton[*MySingleton] = &MySingleton{}

// type MySingleton struct {
// 	instance *MySingleton
// }

// func (s *MySingleton) New() *MySingleton {
// 	if s.instance == nil {
// 		s.instance = &MySingleton{}
// 	}
// 	return s.instance
// }
