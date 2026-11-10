package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// stationProbeRefusals lowers a probe creature carrying lines and returns its
// refusal reasons.
func stationProbeRefusals(t *testing.T, lines ...string) string {
	t.Helper()
	src := strings.Join(append([]string{"Name:Probe", "ManaCost:1 G", "Types:Creature Bear", "PT:2/2"}, lines...), "\n")
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	var out []string
	for _, r := range lowerPool(p).cards[0].Refusals {
		out = append(out, r.Reason)
	}
	return strings.Join(out, "; ")
}

// TestUngatedFixedColourFilterStaysRefused: no corpus card prints a second
// "{G}, {T}: Add {G} for each ..." beside a free tap ability (the other
// coloured-fee filters are hybrid, any-colour, a different colour or a literal
// amount), so the same shape is probed inline. It is a printed census gap the
// kernel's gate view does not price, and refused; so is a Station-gated one
// whose fee is not the colour it makes, has two pips or a generic part, or a
// literal amount, and a Station grant of any other costed mana ability.
func TestUngatedFixedColourFilterStaysRefused(t *testing.T) {
	for _, c := range []struct {
		name, want string
		lines      []string
	}{
		{"ungated {G},{T}: Add {G} per creature", "2 mana abilities: mana ability 1",
			[]string{"A:AB$ Mana | Cost$ T | Produced$ G", "A:AB$ Mana | Cost$ G T | Produced$ G | Amount$ X", "SVar:X:Count$Valid Creature.YouCtrl"}},
		{"Station-gated {G},{T}: Add {U} per creature (fee is not the product)", "granted mana ability with a mana fee",
			stationProbe("AB$ Mana | Cost$ G T | Produced$ U | Amount$ X")},
		{"Station-gated {1},{T}: Add {G} per creature (generic fee)", "granted mana ability with a mana fee",
			stationProbe("AB$ Mana | Cost$ 1 T | Produced$ G | Amount$ X")},
		{"Station-gated {G}{G},{T}: Add {G} per creature (two pips)", "granted mana ability with a mana fee",
			stationProbe("AB$ Mana | Cost$ G G T | Produced$ G | Amount$ X")},
		{"Station-gated {G},{T}: Add {G} (literal amount)", "a coloured-fee mana ability the kernel does not model",
			stationProbe("AB$ Mana | Cost$ G T | Produced$ G | Amount$ 2")},
		{"Station-granted {T}, Sacrifice mana ability", "granted mana ability whose cost is not {T}",
			stationProbe("AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ G")},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := stationProbeRefusals(t, c.lines...)
			if got == "" || !strings.Contains(got, c.want) {
				t.Errorf("want a refusal containing %q, got %q", c.want, got)
			}
		})
	}
}

func stationProbe(grant string) []string {
	return []string{
		"K:Station:12",
		"A:AB$ Mana | Cost$ T | Produced$ G",
		"S:Mode$ Continuous | AffectedDefined$ Self | Affected$ Card.counters_GE12_CHARGE | AddAbility$ ProbeMana",
		"SVar:ProbeMana:" + grant,
		"SVar:X:Count$Valid Creature.YouCtrl",
	}
}
