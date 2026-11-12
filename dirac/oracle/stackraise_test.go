package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Kaervek's Torch: "As long as CARDNAME is on the stack, spells that target it
// cost {2} more to cast" (S:Mode$ RaiseCost | ValidTarget$ Spell.Self | Type$
// Spell | Amount$ 2 | EffectZone$ Stack). The raise lives on the Torch's own
// spell on the stack, so the kernel prices it from the stack source and the
// cast's chosen targets (cost_raise.go raiseStackSelfGen); the expected answer
// at every decision is gorge's.
var stackraiseDecks = [2][]cardset.Entry{
	{
		{Name: "Kaervek's Torch", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Mountain", Count: 24},
	},
	{
		{Name: "Counterspell", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Island", Count: 24},
	},
}

// stackraiseStats counts, in gorge's event log, the Torches cast and the
// spells that chose a Torch spell as a target while it was on the stack.
type stackraiseStats struct{ torches, targeted int }

func stackraiseScan(g *GorgeSide, st *stackraiseStats) {
	if g == nil {
		return
	}
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.PutOnStack:
			if chName(g.E.G, ev.Obj) == "Kaervek's Torch" {
				st.torches++
			}
		case events.TargetsChosen:
			if ev.Amount != 0 || chName(g.E.G, ev.Obj) == "Kaervek's Torch" {
				continue // player targets and the Torch's own damage targets
			}
			for _, id := range ev.IDs {
				if chName(g.E.G, id) == "Kaervek's Torch" {
					st.targeted++
					break
				}
			}
		}
	}
}

// TestReproStackRaiseLockstep plays both seats' decks in lockstep on both
// surfaces over the generated stackraise pool: no divergence, a Torch cast,
// and at least one spell targeting it (so the raise is charged on both sides).
func TestReproStackRaiseLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("stackraise")
	if err != nil {
		t.Fatal(err)
	}
	games, divs, stalls := 0, 0, 0
	st := stackraiseStats{}
	for swap := 0; swap < 2; swap++ {
		md := stackraiseDecks
		if swap == 1 {
			md = [2][]cardset.Entry{stackraiseDecks[1], stackraiseDecks[0]}
		}
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("swap %d deck %d: %d cards", swap, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 60; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kstackraise-a", "kstackraise-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			inner := eagerPolicy(seed, map[string]int{})
			pol := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					gs = c.G
				}
				return inner.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 25})
			CloseSide(k)
			games++
			stackraiseScan(gs, &st)
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				t.Errorf("swap %d seed %d: kernel stall %q", swap, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, %d divergences, %d stalls; %d Torches cast, %d spells targeted a Torch", games, divs, stalls, st.torches, st.targeted)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.torches < 1 {
		t.Fatal("no Kaervek's Torch was cast: the lockstep never reaches the raise")
	}
	if st.targeted < 1 {
		t.Fatal("no spell targeted a Torch on the stack: the raise was never charged")
	}
}
