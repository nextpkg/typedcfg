module example.org/env-demo

go 1.26.0

require (
	github.com/nextpkg/typedcfg v1.0.0
	github.com/pelletier/go-toml/v2 v2.2.4
	go.yaml.in/yaml/v3 v3.0.6
)

replace github.com/nextpkg/typedcfg => ../..
