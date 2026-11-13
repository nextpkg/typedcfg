package main

import (
	"fmt"
	"os"

	"github.com/nextpkg/typedcfg"
)

type Config struct {
	Port int `json:"port" default:"8080"`
}

func main() {
	if len(os.Args) != 2 {
		panic("usage: basic config.json")
	}
	cm := typedcfg.MustLoad[Config](os.Args[1])
	defer func() {
		if err := cm.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	fmt.Println(cm.Get().Port)
}
