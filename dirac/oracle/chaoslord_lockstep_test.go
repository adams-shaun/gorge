package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Chaos Lord's upkeep trigger hands the Lord to a targeted opponent when the
// number of permanents is even: the GainControl body is gated on an SVar
// holding the permanent count mod 2, compared equal to zero. This plays the
// two-seat deck the divergence was measured on (8 Chaos Lord, 10 Hill Giant,
// 22 Mountain on both sides, every offered attacker declared, casting Chaos
// Lord then Hill Giant) against gorge decision by decision: a parity read that
// only the kernel gets wrong shows up as a control change on one side alone.
//
// The table is built from the card names alone (printedDelveSet), so a
// spelling kgen still refuses fails the case at Matchup.

func chaosLordDecks() [2][]string {
	deck := func() []string {
		return append(deckOf(8, "Chaos Lord", deckOf(10, "Hill Giant")...), deckOf(22, "Mountain")...)
	}
	return [2][]string{deck(), deck()}
}

// chaosLordReached reports whether a Chaos Lord was handed over in g's log:
// a ControlChange event on a Lord's object naming a player other than its
// owner (the trigger's GainControl resolving; a later hand-back does not
// unreach it).
func chaosLordReached(g *GorgeSide) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.ControlChange {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil || o.Face().Name != "Chaos Lord" {
			continue
		}
		if e.Player != o.Owner {
			return true
		}
	}
	return false
}

// chaosLordRun plays the case's seeds in lockstep and returns the games and
// the games that reached the control change.
func chaosLordRun(t *testing.T, decks [2][]string, seeds uint64) (games, hits int) {
	t.Helper()
	reg := synthReg(t)
	set := printedDelveSet(t, decks)
	var ds [2][]*cards.Card
	for s := range ds {
		for _, n := range decks[s] {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			ds[s] = append(ds[s], card)
		}
	}
	entries := []cardset.Entry{{Name: "Chaos Lord", Count: 8}, {Name: "Hill Giant", Count: 10}, {Name: "Mountain", Count: 22}}
	tbl, _, _, err := set.Matchup(entries, entries)
	if err != nil {
		t.Fatalf("the pool does not lower: %v", err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: true}
		m, err := NewMatchup(reg, [2]string{"chaos_lord-a", "chaos_lord-b"}, ds, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 30}
		reached := false
		rc.Finish = func(g *GorgeSide) { reached = chaosLordReached(g) }
		base := effectLongPolicy(m, effectLongCase{prefer: []string{"Chaos Lord", "Hill Giant"}}, seed)
		// every offered attacker is declared (blocking stays random): the
		// sick creature a static exempts is rarely sent otherwise.
		pol := PolicyFunc(func(p *PickCtx) Canon {
			if p.D.Kind == DAttackers {
				for _, o := range p.D.Options {
					if o.Kind() == ActAttack {
						return o
					}
				}
			}
			return base.Pick(p)
		})
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if reached {
			hits++
		}
	}
	return games, hits
}

func TestChaosLordLockstep(t *testing.T) {
	decks := chaosLordDecks()
	games, hits := chaosLordRun(t, decks, 60)
	if hits == 0 {
		t.Fatalf("%d games, none reached the control change", games)
	}
}
