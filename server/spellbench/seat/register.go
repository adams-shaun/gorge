package seat

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/seat"
)

var policyName = regexp.MustCompile(`^[a-z0-9-]+$`)

// Register makes name available to the bot registry as a Spellbench v2 agent
// launched from argv. The caller validates the name and path first; a
// duplicate registration is ignored so an in-process test can register the
// same fixture more than once.
func Register(name string, argv []string) error {
	if !policyName.MatchString(name) {
		return fmt.Errorf("spellbench seat %q: invalid name (want ^[a-z0-9-]+$)", name)
	}
	if len(argv) == 0 {
		return fmt.Errorf("spellbench seat %q: empty command", name)
	}
	if _, ok := bots.Lookup(name); ok {
		return nil
	}
	cmd := append([]string(nil), argv...)
	bots.Register(bots.Entry{
		Info: bots.Info{
			Name:         name,
			Label:        "SpellBench v2 agent",
			Description:  "External SpellBench v2 stdio-NDJSON agent",
			Tier:         bots.Experimental,
			Formats:      []string{"constructed"},
			MaxSeats:     2,
			NeedsAutoPay: true,
		},
		New: func(o bots.Options) (seat.Seat, error) {
			return New(name, cmd, o)
		},
	})
	return nil
}

// ParseSpec parses a "name=/path/to/command" -sb-agent value. The command is
// exec'd directly with no shell and no arguments, as the gorge-agent/1 rule
// requires; a wrapper script carries any arguments.
func ParseSpec(spec string) (string, []string, error) {
	name, cmd, ok := strings.Cut(spec, "=")
	if !ok || name == "" || strings.TrimSpace(cmd) == "" {
		return "", nil, fmt.Errorf("want name=/path/to/agent, got %q", spec)
	}
	return name, []string{cmd}, nil
}
