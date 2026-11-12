package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Chain of Vapor bounces a nonland permanent, then that permanent's
// controller may sacrifice a land and, if they do, copy the spell (and may
// choose a new target for the copy). UnlessAI$ is Forge's AI hint for that
// copy and no rules; Indulgent Tormentor's upkeep trigger lets the targeted
// opponent pay 3 life or sacrifice a creature to stop the draw. Both sides
// play the same mirror deck decision by decision: before the hint is ignored
// neither card is in the pool.
func TestChainOfVaporLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	d := []cardset.Entry{
		{Name: "Chain of Vapor", Count: 8}, {Name: "Indulgent Tormentor", Count: 3},
		{Name: "Merfolk of the Pearl Trident", Count: 3}, {Name: "Walking Corpse", Count: 3},
		{Name: "Control Magic", Count: 4}, {Name: "Island", Count: 10}, {Name: "Swamp", Count: 9},
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("chainofvapor")
	if err != nil {
		t.Skip(err) // no tables yet: make dirac-tables
	}
	var deck []*cards.Card
	for _, e := range d {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	decks := [2][]*cards.Card{deck, deck}
	tbl, _, _, err := set.Matchup(d, d)
	if err != nil {
		t.Fatal(err)
	}
	var copies, otherCtrl, stolen, tormentorAsks int
	for seed := uint64(1); seed <= 400; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"cov-a", "cov-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var cur pkEvents
		var lastD any
		check := func(ref *GorgeSide, _ Side) *Divergence {
			g := ref.E.G
			if p := ref.E.Pending(); p != nil && any(p) != lastD {
				lastD = p
				if pkFaceName(g.Obj(p.Source)) == "Indulgent Tormentor" {
					tormentorAsks++
				}
			}
			cur.each(ref, func(e *events.Event) {
				switch e.Kind {
				case events.StackCopy:
					if src := g.Obj(e.Obj); src != nil && pkFaceName(src) == "Chain of Vapor" {
						copies++
						if src.Controller != e.Player {
							otherCtrl++
						}
						// the copier is not the bounced permanent's owner: it was
						// stolen, and its last-known controller (not the owner the
						// bounce reset it to) sacrificed the land (CR 608.2h)
						if len(src.Targets) > 0 && !src.Targets[0].IsPlayer {
							if tg := g.Obj(src.Targets[0].Obj); tg != nil && tg.Owner != e.Player {
								stolen++
							}
						}
					}
				}
			})
			return nil
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30, Check: check})
		CloseSide(k)
		if dv := res.Div; dv != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, dv.At, dv.What, dv.Field, dv.Diff, dv.Detail, dv.RefOptions, dv.KerOptions, res.RecentSources(m, 6))
		}
	}
	t.Logf("Chain of Vapor copies %d (by the other player %d, of a stolen permanent %d), Tormentor asks %d", copies, otherCtrl, stolen, tormentorAsks)
	if copies < 5 || otherCtrl < 1 || stolen < 1 || tormentorAsks < 5 {
		t.Errorf("too little coverage: copies %d (other controller %d, stolen %d), Tormentor asks %d", copies, otherCtrl, stolen, tormentorAsks)
	}
}
