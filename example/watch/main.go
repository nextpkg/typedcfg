package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/nextpkg/typedcfg"
)

type Config struct {
	Port int `json:"port" default:"8080" validate:"min=1,max=65535"`
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run ./example/watch <config.yaml>")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	cm, err := typedcfg.NewBuilder[Config]().AddFile(os.Args[1]).WithCreateIfMissing().WithWatch().Build(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := cm.Close(); err != nil {
			log.Printf("close configuration: %v", err)
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-cm.Errors():
			if !ok {
				return
			}
			log.Printf("reload failed: %v", err)
		case <-ticker.C:
			fmt.Printf("port=%d\n", cm.Get().Port)
		}
	}
}
