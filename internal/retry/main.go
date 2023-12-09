package main

import (
	"fmt"
	"log/slog"
	"math"
	"time"
)

func main() {
	if err := Retry(func() error {
		slog.Info("Doing stuff")

		return nil
	}); err != nil {
		panic(err)
	}
}

func Retry(
	fn func() error,
) error {
	attempt := 0

	for {
		slog.Info(fmt.Sprintf("Attempt %d", attempt))

		// 初回の実行 + 3回のリトライ
		if attempt > 4 {
			return fmt.Errorf("failed after %d attempts", attempt)
		}

		if err := fn(); err == nil {
			return nil
		} else {
			slog.Warn(err.Error())
		}

		interval := int(math.Pow(2, float64(attempt)))

		time.Sleep(time.Duration(interval) * time.Second)

		attempt++
	}
}
