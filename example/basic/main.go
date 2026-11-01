package main

import (
	"fmt"
	"os"

	"github.com/nextpkg/typedcfg"
)

type Config struct {
	Port int `json:"port" default:"8080" validate:"min=1,max=65535"`
}

func main() {
	cfg := typedcfg.MustLoad[Config](os.Args[1:]...).Get()
	fmt.Printf("port=%d\n", cfg.Port)
}
