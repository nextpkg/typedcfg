package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/nextpkg/typedcfg"
	"github.com/nextpkg/typedcfg/adapters/cliv3"
	"github.com/urfave/cli/v3"
)

type Config struct {
	Server struct {
		Port int `json:"port" default:"8080" validate:"min=1,max=65535"`
	} `json:"server"`
	Enabled bool `json:"enabled" default:"true"`
}

func main() {
	cmd := &cli.Command{Name: "typedcfg-example", Flags: []cli.Flag{
		&cli.IntFlag{Name: "server.port"}, &cli.BoolFlag{Name: "enabled"},
	}, Action: func(ctx context.Context, cmd *cli.Command) error {
		cm, err := typedcfg.NewBuilder[Config]().AddEnv("APP_").
			AddProvider(cliv3.Provider(cmd, "server.port", "enabled"), nil).Build(ctx)
		if err != nil {
			return err
		}
		defer func() {
			if err := cm.Close(); err != nil {
				log.Printf("close configuration: %v", err)
			}
		}()
		fmt.Printf("port=%d enabled=%t\n", cm.Get().Server.Port, cm.Get().Enabled)
		return nil
	}}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
