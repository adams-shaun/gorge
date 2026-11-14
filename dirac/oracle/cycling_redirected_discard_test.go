package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Gempalm Incinerator's "When you cycle CARDNAME" beside Rest in Peace (MBC
// pool, dr-mbc-aa pair 53): the cycling cost discard is redirected into
// exile, and the Cycled trigger still fires (CR 614.6, 702.29d). gorge lost
// the trigger when the redirected move's Counter carried the exiler stamp in
// place of the cycling cause, one stack entry behind the kernel. A mirror of
// both cards, played in lockstep, must stay clean, and some game must see the
// trigger put on gorge's stack while the cycled card sits in exile.

var cycledRedirectDeck = []cardset.Entry{
	{Name: "Gempalm Incinerator", Count: 8}, {Name: "Rest in Peace", Count: 4}, {Name: "Goblin King", Count: 3},
	{Name: "Mogg Raider", Count: 3}, {Name: "Mountain", Count: 14}, {Name: "Plains", Count: 8},
}

func TestCycledRedirectedDiscardLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	const pool = "mbc"
	decks := [2][]cardset.Entry{cycledRedirectDeck, cycledRedirectDeck}
	reg, set, gd := lockstepPool(t, pool, decks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Fatalf("seat %d: the deck is not playable in the %s pool", s, pool)
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	reached := 0
	for seed := uint64(1); seed <= 40; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{pool + "-a", pool + "-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castActivatePolicy(seed)
		if seed%4 == 3 {
			pol = RandomPolicy(seed)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
		hit, next := false, 0
		rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; next < len(evs); next++ {
				e := evs[next]
				if e.Kind != events.TriggerPush || !isCardNamed(ref, e.Obj, []string{"Gempalm Incinerator"}) {
					continue
				}
				// Gempalm's only trigger is the Cycled one; its source in
				// exile means the cycling discard was redirected there.
				if o := ref.E.G.Obj(e.Obj); o != nil && o.Zone == state.ZExile {
					hit = true
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			from := max(d.At-12, 0)
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nlast actions %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, actionStrings(res.Actions[from:d.At]))
		}
		if hit {
			reached++
		}
	}
	if reached == 0 {
		t.Fatal("no game put Gempalm Incinerator's Cycled trigger on gorge's stack from exile: the mirror does not exercise the redirect")
	}
	t.Logf("%d of 40 games saw the redirected Cycled trigger", reached)
}
