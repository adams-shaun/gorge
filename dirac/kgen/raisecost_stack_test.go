package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Kaervek's Torch: "As long as CARDNAME is on the stack, spells that target
// it cost {2} more to cast" (RaiseCost | ValidTarget$ Spell.Self | Type$ Spell
// | Amount$ 2 | EffectZone$ Stack). The source is the spell on the stack, so
// the lowered static is an IncreaseCost with Zone ZoneStack; its target gate
// reads a Spell.Self stack filter (CR 113.6, 601.2f).
func TestRaiseCostStackSelfLowering(t *testing.T) {
	all, bs, pt, err := buildPool(lowerNamedPool(t, []string{"Kaervek's Torch"}))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for i, c := range all {
		if c.Name != "Kaervek's Torch" {
			continue
		}
		if r := lowerRefusals(c); len(r) != 0 {
			t.Fatalf("refused: %v", r)
		}
		for _, a := range bs[i].def.Abilities {
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatIncreaseCost {
				continue
			}
			found++
			st := a.Static
			if st.Zone != kop.ZoneStack || st.A != 2 || st.StFlags&kop.StTargetsFilter2 == 0 {
				t.Errorf("static A %d Zone %v flags %#x, want A 2 Zone ZoneStack StTargetsFilter2", st.A, st.Zone, st.StFlags)
			}
			if st.Filter2 == 0 {
				t.Fatal("no Filter2: the Spell.Self target gate did not lower")
			}
			f := pt.filters[st.Filter2]
			if f.Zone != kop.ZoneStack || f.Flags&kop.FSelf == 0 {
				t.Errorf("Filter2 zone %v flags %#x, want ZoneStack with FSelf", f.Zone, f.Flags)
			}
		}
	}
	if found != 1 {
		t.Fatalf("%d StatIncreaseCost abilities, want 1", found)
	}
}

// raiseCostStackRefusal lowers one synthetic RaiseCost static on a sorcery
// and returns its refusals joined; the probe is a spell so the type gate
// passes and only the static's own shape can refuse.
func raiseCostStackRefusal(t *testing.T, static string) string {
	t.Helper()
	src := strings.Join([]string{"Name:Probe", "ManaCost:X R", "Types:Sorcery", static}, "\n")
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

// TestRaiseCostStackSelfRefusals pins the shapes the Stack arm does not price,
// each under its own reason (never the old blanket "EffectZone Stack" one):
// a Cost$ (Liesa, Shroud of Dusk's PayLife<X>), a ValidCard$ self raise, a
// target other than Spell.Self, and a computed Amount$.
func TestRaiseCostStackSelfRefusals(t *testing.T) {
	for _, c := range []struct{ name, static, want string }{
		{"pay-life cost", "S:Mode$ RaiseCost | ValidCard$ Card.Self+wasCastFromCommand | Type$ Spell | Cost$ PayLife<X> | EffectZone$ Stack", `RaiseCost Cost$ "PayLife<X>"`},
		{"self card raise", "S:Mode$ RaiseCost | ValidCard$ Card.Self | ValidTarget$ Spell.Self | Type$ Spell | Amount$ 2 | EffectZone$ Stack", "with ValidCard$"},
		{"permanent target", "S:Mode$ RaiseCost | ValidTarget$ Creature | Type$ Spell | Amount$ 2 | EffectZone$ Stack", "without ValidTarget$ Spell.Self"},
		{"computed amount", "S:Mode$ RaiseCost | ValidTarget$ Spell.Self | Type$ Spell | Amount$ X | EffectZone$ Stack\nSVar:X:Count$YourTurns", "non-literal Amount$"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := raiseCostStackRefusal(t, c.static)
			if !strings.Contains(got, c.want) {
				t.Errorf("want refusal containing %q, got %q", c.want, got)
			}
		})
	}
}
