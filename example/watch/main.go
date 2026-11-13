package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/nextpkg/typedcfg"
)

type Config struct {
	Port int `json:"port" default:"8080"`
}

func main() {
	if len(os.Args) != 2 {
		panic("usage: watch config.json")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cm, err := typedcfg.LoadContext[Config](ctx, os.Args[1], typedcfg.WithCreateIfMissing(), typedcfg.WithWatch())
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := cm.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	fmt.Println("Initial port:", cm.Get().Port)
	for err := range cm.Errors() {
		fmt.Fprintln(os.Stderr, err)
	}
}
