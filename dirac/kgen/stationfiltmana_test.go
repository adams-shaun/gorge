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
// literal amount, and a Station grant of any other costed mana ability (the
// granted {T}, Sacrifice shape lowered with the tap-and-sacrifice grant, so
// its case asserts the lowering).
func TestUngatedFixedColourFilterStaysRefused(t *testing.T) {
	for _, c := range []struct {
		name, want string
		lines      []string
	}{
		{"ungated {G},{T}: Add {G} per creature", "2 mana abilities: mana ability 1",
			[]string{"A:AB$ Mana | Cost$ T | Produced$ G", "A:AB$ Mana | Cost$ G T | Produced$ G | Amount$ ProbeCount", "SVar:ProbeCount:Count$Valid Creature.YouCtrl"}},
		{"Station-gated {G},{T}: Add {U} per creature (fee is not the product)", "granted mana ability with a mana fee",
			stationProbe("AB$ Mana | Cost$ G T | Produced$ U | Amount$ ProbeCount")},
		{"Station-gated {1},{T}: Add {G} per creature (generic fee)", "granted mana ability with a mana fee",
			stationProbe("AB$ Mana | Cost$ 1 T | Produced$ G | Amount$ ProbeCount")},
		{"Station-gated {G}{G},{T}: Add {G} per creature (two pips)", "granted mana ability with a mana fee",
			stationProbe("AB$ Mana | Cost$ G G T | Produced$ G | Amount$ ProbeCount")},
		{"Station-gated {G},{T}: Add {G} (literal amount)", "a coloured-fee mana ability the kernel does not model",
			stationProbe("AB$ Mana | Cost$ G T | Produced$ G | Amount$ 2")},
		{"Station-granted {T}, Sacrifice mana ability (the granted tap-and-sacrifice shape lowers; dirac mana_grantcost.go)", "",
			stationProbe("AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ G")},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := stationProbeRefusals(t, c.lines...)
			if c.want == "" {
				if got != "" {
					t.Errorf("a supported shape is refused: %s", got)
				}
				return
			}
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
		"SVar:ProbeCount:Count$Valid Creature.YouCtrl",
	}
}

// TestStationLowersWithOrWithoutItsNumber: upstream Forge dropped the number
// from K:Station:<N> (the threshold lives on the face's STATION N+ static),
// and gorge offers Station either way, so the kernel lowers the Station
// ability for both spellings. 11-20: the bare K:Station lowered no ability,
// and std-trk diverged in 1009 of 1500 games ("options vs options").
func TestStationLowersWithOrWithoutItsNumber(t *testing.T) {
	for _, kw := range []string{"K:Station", "K:Station:8"} {
		src := strings.Join([]string{"Name:Probe", "ManaCost:1 R", "Types:Artifact Spacecraft", "PT:3/3", kw,
			"S:Mode$ Continuous | AffectedDefined$ Self | Affected$ Card.counters_GE8_CHARGE | AddType$ Creature | AddKeyword$ Flying"}, "\n")
		card, diags := cards.ParseBytes("Probe.txt", []byte(src))
		if card == nil || len(diags) != 0 {
			t.Fatalf("%s: parse: %v", kw, diags)
		}
		if d := card.Link(); len(d) != 0 {
			t.Fatalf("%s: link: %v", kw, d)
		}
		p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
			nDecks: map[string]int{}, reg: &cards.Registry{}}
		lc := lowerPool(p).cards[0]
		if len(lc.Refusals) != 0 {
			t.Fatalf("%s: refused: %v", kw, lc.Refusals)
		}
		n := 0
		for _, a := range lc.Abilities {
			if a.Station {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s: %d Station abilities, want 1", kw, n)
		}
	}
}
