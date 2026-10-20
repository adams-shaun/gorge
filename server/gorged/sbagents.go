package gorged

import (
	"fmt"
	"os/exec"
	"regexp"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/server/spellbench/seat"
)

type sbAgentSpec struct {
	name string
	argv []string
}

var sbAgentName = regexp.MustCompile(`^[a-z0-9-]+$`)

// parseSBAgentSpec validates one -sb-agent value at flag-parse time: the name
// is a registry-safe slug and the command exists as an executable file.
func parseSBAgentSpec(arg string) (sbAgentSpec, error) {
	name, argv, err := seat.ParseSpec(arg)
	if err != nil {
		return sbAgentSpec{}, err
	}
	if !sbAgentName.MatchString(name) {
		return sbAgentSpec{}, fmt.Errorf("-sb-agent: invalid name %q (want ^[a-z0-9-]+$)", name)
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return sbAgentSpec{}, fmt.Errorf("-sb-agent %s: %w", name, err)
	}
	return sbAgentSpec{name: name, argv: argv}, nil
}

// registerSBAgents adds every -sb-agent to the bots registry. A duplicate name
// or a collision with a linked policy is a startup error, not a panic from
// bots.Register.
func registerSBAgents(specs []sbAgentSpec) error {
	seen := map[string]bool{}
	for _, spec := range specs {
		if seen[spec.name] {
			return fmt.Errorf("-sb-agent: duplicate name %q", spec.name)
		}
		seen[spec.name] = true
		if _, ok := bots.Lookup(spec.name); ok {
			return fmt.Errorf("-sb-agent: %q is already a registered bot policy", spec.name)
		}
		if err := seat.Register(spec.name, spec.argv); err != nil {
			return err
		}
	}
	return nil
}
