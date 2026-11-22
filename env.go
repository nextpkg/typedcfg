package typedcfg

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// WithEnvExpansion controls expansion of ${NAME} in decoded string values and
// in numeric and boolean default tags before they are parsed.
func WithEnvExpansion(enabled bool) Option { return func(o *options) { o.expandEnv = enabled } }

var nameRE = regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_]*$")

func expandEnvironment(s string) (string, error) {
	if !strings.Contains(s, "${") {
		return s, nil
	}
	var out strings.Builder
	for {
		prefix, tail, found := strings.Cut(s, "${")
		out.WriteString(prefix)
		if !found {
			return out.String(), nil
		}
		name, rest, closed := strings.Cut(tail, "}")
		if !closed || !nameRE.MatchString(name) {
			return "", fmt.Errorf("invalid environment placeholder")
		}
		value, ok := os.LookupEnv(name)
		if !ok || value == "" {
			return "", fmt.Errorf("empty or unset environment variable %s", name)
		}
		out.WriteString(value)
		// Resume in the original template so a value containing ${...} stays literal.
		s = rest
	}
}
