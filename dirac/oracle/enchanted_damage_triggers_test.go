package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEnchantedDamageTriggersLockstep plays the Auras whose trigger watches the
// enchanted creature's damage (ValidSource$ Card.AttachedBy: Vampiric Link and
// Spirit Link's DamageDealtOnce, Curiosity's DamageDone to an opponent) beside
// Walking Ballista, a creature that pings, and Shock, a spell that deals damage
// while an Aura sits on a creature. The kernel used to read the Aura's source
// filter with a card matcher that ignores the host flag, so every damaging
// spell fired every such Aura (dr-mbc-abaa pair 42: Tendrils of Corruption
// fired a Vampiric Link on Typhoid Rats; dirac/trig_hostsource.go). The
// mechanism counts a Shock on gorge's stack while one of the Auras is attached.
func TestEnchantedDamageTriggersLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	auras := []string{"Vampiric Link", "Spirit Link", "Curiosity"}
	deck := []cardset.Entry{
		{Name: "Walking Ballista", Count: 8}, {Name: "Vampiric Link", Count: 4}, {Name: "Spirit Link", Count: 4},
		{Name: "Curiosity", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Swamp", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4},
	}
	bears := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}}
	decks := [2][]cardset.Entry{deck, bears}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			names[s] = append(names[s], e.Name)
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				cs[s] = append(cs[s], c)
			}
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	isAura := func(o *state.Object) bool {
		if o == nil || o.Face() == nil || o.AttachedTo == 0 {
			return false
		}
		for _, n := range auras {
			if o.Face().Name == n {
				return true
			}
		}
		return false
	}
	// spellWithAura: a Shock is on the stack while an Aura of the three is
	// attached to a permanent (the path the fix closes).
	spellWithAura := func(ref *GorgeSide) bool {
		g := ref.E.G
		shock := false
		for _, id := range g.Stack {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Shock" {
				shock = true
			}
		}
		if !shock {
			return false
		}
		for p := range state.PlayerID(2) {
			for _, id := range g.Zone(state.ZBattlefield, p) {
				if isAura(g.Obj(id)) {
					return true
				}
			}
		}
		return false
	}
	reached, fired := 0, 0
	const seeds = 24
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"enchdmg-a", "enchdmg-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		hit, trig := false, false
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
		rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
			hit = hit || spellWithAura(ref)
			if !trig {
				g := ref.E.G
				for _, id := range g.Stack {
					// a triggered ability on the stack whose source is one of the Auras
					if o := g.Obj(id); o != nil && o.Face() == nil {
						if src := g.Obj(o.Source); src != nil && src.Face() != nil {
							for _, n := range auras {
								trig = trig || src.Face().Name == n
							}
						}
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		if hit {
			reached++
		}
		if trig {
			fired++
		}
	}
	t.Logf("a damaging spell with an Aura attached in %d of %d games; an Aura's damage trigger on the stack in %d", reached, seeds, fired)
	if reached < 3 {
		t.Errorf("a damaging spell with an Aura attached in %d of %d games, want at least 3", reached, seeds)
	}
	if fired < 3 {
		t.Errorf("an Aura's damage trigger on the stack in %d of %d games, want at least 3", fired, seeds)
	}
}
