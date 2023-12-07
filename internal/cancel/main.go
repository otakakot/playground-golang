package main

import (
	"context"
	"log"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	done := make(chan struct{})

	log.Print("start")

	go Done(ctx, done)

	Cancel(cancel)

	<-ctx.Done()

	log.Printf("%+v", ctx.Err())

	log.Print("end")

	<-done // Done が返ってくるまでスレッド止めておく

	log.Print("done")
}

func Done(ctx context.Context, done chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("context done")
			done <- struct{}{}
			return
		default:
			log.Printf("context not done")
			time.Sleep(time.Second)
		}
	}
}

func Cancel(fn func()) {
	time.Sleep(5 * time.Second)

	log.Printf("context cancel")

	fn()
}

// cancel() ってなんでキャンセルなの？
// -> context.Context をキャンセルするメソッドだから

// じゃあ、なんで defer cancel() なんてするの？
// -> cancel() を実行することによってリソースを解放したいため
