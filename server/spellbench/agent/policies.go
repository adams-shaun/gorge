package agent

import (
	"fmt"
	"runtime/debug"

	"github.com/adams-shaun/gorge/bots"
)

// Policy identifies one registered bot policy. The benchmark label is derived
// from the registry key so every shipped policy keeps a stable gorge identity.
type Policy struct {
	Key, Name string
	Info      bots.Info
}

func lookupPolicy(key string) (Policy, error) {
	key, err := bots.Normalize(key)
	if err != nil {
		return Policy{}, err
	}
	entry, ok := bots.Lookup(key)
	if !ok {
		return Policy{}, fmt.Errorf("unknown or not yet integrated policy %q", key)
	}
	return Policy{Key: key, Name: "gorge-" + key, Info: entry.Info}, nil
}

// Version reports the gorge build revision so a Spellbench run names the exact
// binary that played it.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "devel"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" && setting.Value != "" {
			return setting.Value
		}
	}
	return "devel"
}

func (p Policy) outOfScope() bool { return p.Info.Env || p.Info.Search }

func (p Policy) decklistRefusal() string {
	return fmt.Sprintf("policy %q reads every seat's real decklist and needs rules.opponent_decklist \"visible\"", p.Key)
}

func (p Policy) scopeRefusal() string {
	return fmt.Sprintf("policy %q needs an engine root (Env/Search) and is not supported by this agent", p.Key)
}

func (p Policy) autoPayRefusal() string {
	return fmt.Sprintf("policy %q needs engine auto-pay, but the engine offers manual mana payment", p.Key)
}
