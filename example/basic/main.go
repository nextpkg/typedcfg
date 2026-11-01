package main

import (
	"fmt"
	"log"

	"github.com/nextpkg/typedcfg"
)

type Config struct {
	Port int `json:"port" default:"8080" validate:"min=1,max=65535"`
}

func main() {
	cm, err := typedcfg.Load[Config]()
	if err != nil {
		log.Fatal(err)
	}
	defer cm.Close()
	fmt.Printf("port=%d\n", cm.Get().Port)
}
