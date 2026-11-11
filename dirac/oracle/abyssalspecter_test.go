package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestAbyssalSpecterLockstep: Abyssal Specter's "whenever this deals damage to
// a player, that player discards a card" (a plain DamageDone line to any
// player that reads the damaged player, CR 603.2) plays identically in gorge
// and the kernel: the kernel queues it per referent, so the discarding player
// is the damaged one. The table is built from the decks themselves; the
// census requires the trigger to fire, so a game that never connects is not a
// pass.
func TestAbyssalSpecterLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	deck := []string{}
	for i := 0; i < 8; i++ {
		deck = append(deck, "Abyssal Specter")
	}
	for i := 0; i < 8; i++ {
		deck = append(deck, "Grizzly Bears", "Lightning Bolt")
	}
	for len(deck) < 40 {
		deck = append(deck, "Swamp")
	}
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
	fired := 0
	for seed := uint64(1); seed <= 24; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"specter-a", "specter-b"}, cardDecks, 1)
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
				if ev.Kind != events.TriggerPush {
					continue
				}
				o := ref.E.G.Obj(ev.Obj)
				if o != nil && o.Face() != nil && o.Face().Name == "Abyssal Specter" {
					fired++
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
	if fired < 5 {
		t.Errorf("Abyssal Specter's trigger fired %d times in 24 games, want at least 5", fired)
	}
}
