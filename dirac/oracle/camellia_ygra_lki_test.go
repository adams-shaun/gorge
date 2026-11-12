package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCamelliaYgraLKILockstep: Camellia's "whenever you sacrifice one or
// more Foods" looks back at the sacrificed permanent as it last existed
// (CR 603.10a), so a creature Ygra, Eater of All made a Food still counts
// when it is sacrificed (for Camellia's forage, or Ygra's granted ability).
// Both engines must queue Camellia's trigger for it. The census requires
// such a sacrifice: a Ygra-Food creature that left the battlefield by a
// sacrifice event while Camellia was on the battlefield.
func TestCamelliaYgraLKILockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	var deck []string
	for i := 0; i < 6; i++ {
		deck = append(deck, "Camellia, the Seedmiser", "Ygra, Eater of All")
	}
	for i := 0; i < 8; i++ {
		deck = append(deck, "Grizzly Bears", "Carrot Cake")
	}
	for len(deck) < 40 {
		deck = append(deck, "Swamp", "Forest")
	}
	deck = deck[:40]
	decks := [2][]string{deck, deck}
	set := printedDelveSet(t, decks)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(deck), entries(deck))
	if err != nil {
		t.Fatal(err)
	}
	var cardDecks [2][]*cards.Card
	for s := range decks {
		for _, n := range decks[s] {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			cardDecks[s] = append(cardDecks[s], c)
		}
	}
	sacrificed, fired := 0, 0
	for seed := uint64(1); seed <= 40; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"camellia-a", "camellia-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				ev := evs[seen]
				if ev.Kind == events.TriggerPush {
					if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Camellia, the Seedmiser" {
						fired++
					}
				}
				if ev.Kind != events.MoveZone || !events.IsSacrifice(ev) {
					continue
				}
				o := ref.E.G.Obj(ev.Obj)
				if o != nil && o.Face() != nil && o.Face().IsCreature() && o.Zone != state.ZBattlefield && o.Face().Name == "Grizzly Bears" {
					for _, id := range ref.E.G.Zone(state.ZBattlefield, o.Owner) {
						if c := ref.E.G.Obj(id); c != nil && c.Face() != nil && c.Face().Name == "Camellia, the Seedmiser" {
							sacrificed++
							break
						}
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	t.Logf("camellia fired %d, creature sacrifices %d", fired, sacrificed)
	if fired == 0 || sacrificed == 0 {
		t.Errorf("census: Camellia fired %d times, %d Grizzly Bears sacrificed (Ygra-Food), want both > 0", fired, sacrificed)
	}
}
