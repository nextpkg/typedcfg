// The nacos example reads a configuration from Alibaba Cloud MSE Nacos or Nacos.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/nextpkg/typedcfg"
	"github.com/nextpkg/typedcfg/adapters/nacos"
)

type Config struct {
	Port int `json:"port" default:"8080" validate:"min=1,max=65535"`
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	host, dataID := os.Getenv("NACOS_HOST"), os.Getenv("NACOS_DATA_ID")
	if host == "" || dataID == "" {
		return fmt.Errorf("set NACOS_HOST (hostname only) and NACOS_DATA_ID (YAML configuration)")
	}
	port := uint64(8848)
	if value := os.Getenv("NACOS_PORT"); value != "" {
		var err error
		port, err = strconv.ParseUint(value, 10, 16)
		if err != nil || port == 0 {
			return fmt.Errorf("NACOS_PORT must be an integer between 1 and 65535")
		}
	}
	// An isolated directory avoids existing SDK failover files overriding remote
	// data and removes this example's logs/cache when it exits.
	dir, err := os.MkdirTemp("", "typedcfg-nacos-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	client, err := clients.NewConfigClient(vo.NacosClientParam{
		ServerConfigs: []constant.ServerConfig{{IpAddr: host, Port: port}},
		ClientConfig: &constant.ClientConfig{
			NamespaceId: os.Getenv("NACOS_NAMESPACE_ID"),
			AccessKey:   os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"),
			SecretKey:   os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET"),
			TimeoutMs:   5000, NotLoadCacheAtStart: true, DisableUseSnapShot: true,
			LogDir: filepath.Join(dir, "log"), CacheDir: filepath.Join(dir, "cache"),
			LogLevel: "error",
		},
	})
	if err != nil {
		return err
	}
	defer client.CloseClient()
	cm := typedcfg.NewBuilder[Config]().
		AddProvider(nacos.Provider(client, dataID, os.Getenv("NACOS_GROUP")), yaml.Parser()).
		AddEnv("APP_").MustBuild()
	defer func() { _ = cm.Close() }()
	fmt.Println(cm.Get().Port)
	return nil
}
