package main

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
)

func main() {
	Hoge(context.Background())
}

func StartEndLog(
	ctx context.Context,
) func() {
	var pc uintptr
	for i := 0; i <= 1; i++ {
		pc, _, _, _ = runtime.Caller(i)
		// 0 ->　StartEndLog
		// 1 ->　呼び出し元
	}

	fn := runtime.FuncForPC(pc)

	slog.InfoContext(ctx, fmt.Sprintf("start %s", fn.Name()))

	end := func() {
		slog.InfoContext(ctx, fmt.Sprintf("end %s", fn.Name()))
	}

	return end
}

func Hoge(ctx context.Context) {
	end := StartEndLog(context.Background())
	defer end()

	slog.InfoContext(ctx, "hoge")
}
