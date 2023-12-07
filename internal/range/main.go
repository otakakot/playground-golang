package main

import (
	"log"
)

type Test struct {
	ID       string
	Password string
}

func main() {
	tests := []Test{
		{
			ID:       "1",
			Password: "XXXXXXXX",
		},
	}

	pass := "XXXXXXXX"

	verified := 0
	// 検証済みのパスワードをキャッシュする
	verifiedPassword := make(map[string]struct{}, len(tests))
	for _, p := range tests {
		if _, ok := verifiedPassword[string(pass)]; ok {
			// 検証済みのパスワードならスキップする
			tests[verified] = p
			verified++
			continue
		}
		if pass == p.Password {
			tests[verified] = p
			verified++
			// 検証済みのパスワードをキャッシュする
			verifiedPassword[string(pass)] = struct{}{}
		}
	}

	log.Printf("%+v", tests[:verified])

}
