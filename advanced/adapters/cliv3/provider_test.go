package cliv3_test

import (
	"context"
	"testing"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
	typedcfg "github.com/nextpkg/typedcfg/advanced"
	"github.com/nextpkg/typedcfg/advanced/adapters/cliv3"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestFlagsOverrideOnlyWhenSet(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		port    int
		enabled bool
	}{
		{"unset", []string{"app", "serve"}, 9000, true},
		{"root alias and nested false", []string{"app", "-p", "9090", "serve", "--enabled=false"}, 9090, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type Config struct {
				Server struct {
					Port int `json:"port"`
				} `json:"server"`
				Enabled bool `json:"enabled"`
			}
			cmd := &cli.Command{Name: "app", Flags: []cli.Flag{&cli.IntFlag{Name: "server.port", Aliases: []string{"p"}, Value: 8080}}, Commands: []*cli.Command{{Name: "serve", Flags: []cli.Flag{&cli.BoolFlag{Name: "enabled"}}, Action: func(ctx context.Context, cmd *cli.Command) error {
				cm, err := typedcfg.NewBuilder[Config]().
					AddProvider(rawbytes.Provider([]byte(`{"server":{"port":9000},"enabled":true}`)), json.Parser()).
					AddProvider(cliv3.Provider(cmd, "server.port", "enabled"), nil).Build(ctx)
				if err != nil {
					return err
				}
				t.Cleanup(func() { require.NoError(t, cm.Close()) })
				require.Equal(t, tc.port, cm.Get().Server.Port)
				require.Equal(t, tc.enabled, cm.Get().Enabled)
				return nil
			}}}}
			require.NoError(t, cmd.Run(t.Context(), tc.args))
		})
	}
}
