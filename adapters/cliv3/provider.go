// Package cliv3 adapts explicitly selected urfave/cli/v3 flags to typedcfg.
package cliv3

import (
	"errors"
	"fmt"
	"strings"

	"github.com/knadh/koanf/maps"
	"github.com/urfave/cli/v3"
)

// Provider reads selected flags by their canonical names from a parsed command.
// Flags override configuration when cmd.IsSet reports true, including values
// supplied through urfave/cli sources or Set calls. Default Value alone is ignored.
// Dots delimit nested fields; selected names are used without a command prefix.
// Call inside Action and pass the result to AddProvider with a nil parser.
func Provider(cmd *cli.Command, names ...string) *Flags {
	return &Flags{cmd: cmd, names: append([]string(nil), names...)}
}

type Flags struct {
	cmd   *cli.Command
	names []string
}

func (f *Flags) Read() (map[string]any, error) {
	if f.cmd == nil {
		return nil, errors.New("CLI command is nil")
	}
	data := make(map[string]any)
	for _, name := range f.names {
		if name == "" || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") {
			return nil, fmt.Errorf("invalid configuration flag name %q", name)
		}
		if f.cmd.IsSet(name) {
			data[name] = f.cmd.Value(name)
		}
	}
	return maps.Unflatten(data, "."), nil
}

func (f *Flags) ReadBytes() ([]byte, error) { return nil, errors.New("CLI flags require a nil parser") }
