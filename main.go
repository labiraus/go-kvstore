package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	ctx, ctxDone := context.WithCancel(context.Background())
	done := startApi(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	s := <-c
	ctxDone()
	fmt.Println("user got signal: " + s.String() + " now closing")
	<-done
}
