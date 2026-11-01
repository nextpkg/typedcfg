// The example watches a Nacos configuration and prints its current port.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/nextpkg/typedcfg/adapters/nacos"
)

type Config struct {
	Port int `json:"port" default:"8080" validate:"min=1,max=65535"`
}

func main() {
	cm := nacos.MustLoad[Config](os.Getenv("NACOS_ADDRESS"), os.Getenv("NACOS_DATA_ID"))
	defer func() {
		if err := cm.Close(); err != nil {
			log.Printf("close configuration: %v", err)
		}
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
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
			log.Printf("configuration update: %v", err)
		case <-ticker.C:
			fmt.Println(cm.Get().Port)
		}
	}
}
