package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDefilerFaceRoutePoolGateLockstep: Defiler of Flesh's second chance
// (cost_defiler.go, gorge reducedOptionalCastable) prices only the plain hand
// cast and the command-zone cast. gorge's warp recast from exile (Timeline
// Culler) and its face routes (Foulmire Knight's creature cast from the
// adventure zone) are offered at the unreduced cost, so the kernel's pool gate
// for those routes must not take the reduction either. Each case FAILS unless
// some seed casts the card from exile while seat 0 controls a Defiler.
func TestDefilerFaceRoutePoolGateLockstep(t *testing.T) {
	for _, tc := range []struct {
		name, card, file string // file: the corpus script's name when it is not card's
	}{
		{"timeline_culler_warp_recast", "Timeline Culler", "Timeline Culler"},
		{"foulmire_knight_face_cast", "Foulmire Knight", "Foulmire Knight Profane Insight"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decks := [2][]string{
				append(append(deckOf(6, "Defiler of Flesh"), deckOf(12, tc.card)...), deckOf(22, "Swamp")...),
				append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...),
			}
			defilerFaceRouteRun(t, tc.card, tc.file, decks, 30)
		})
	}
}

func defilerFaceRouteRun(t *testing.T, card, file string, decks [2][]string, n uint64) {
	t.Helper()
	dir := filters11Corpus(t, nil, "Defiler of Flesh", file, "Swamp", "Forest", "Grizzly Bears")
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	var ds [2][]*cards.Card
	for s, names := range decks {
		for _, nm := range names {
			c, ok := reg.Lookup(nm)
			if !ok {
				t.Fatalf("%s not in the corpus", nm)
			}
			ds[s] = append(ds[s], c)
		}
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, nm := range names {
			es[i] = cardset.Entry{Name: nm, Count: 1}
		}
		return es
	}
	set := printedDelveSetIn(t, dir, decks)
	tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	reached := 0
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"dfr-a", "dfr-b"}, ds, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var hit bool
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Finish: func(g *GorgeSide) { hit = exileCastBesideDefiler(g, card) }}
		res := Lockstep(m, seed, k, defilerFirstPolicy(m, seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s last %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if hit {
			reached++
		}
	}
	t.Logf("%s: cast from exile beside a Defiler in %d of %d games", card, reached, n)
	if reached == 0 {
		t.Fatalf("%s: no seed cast it from exile while seat 0 controlled a Defiler", card)
	}
}

// defilerFirstPolicy casts Defiler of Flesh whenever seat 0 is offered it,
// seat 1 never attacks, and the rest is the seed's random stream.
func defilerFirstPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Defiler of Flesh" {
					return a
				}
			}
		}
		if p.D.Kind == DAttackers && p.D.Player == 1 {
			for _, a := range p.D.Options {
				if a.Kind() != ActAttack {
					return a
				}
			}
		}
		return rnd.Pick(p)
	})
}

// exileCastBesideDefiler: card was put on the stack from exile while a
// Defiler of Flesh was on seat 0's battlefield.
func exileCastBesideDefiler(g *GorgeSide, card string) bool {
	defilers := map[state.ObjID]bool{}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		switch {
		case e.Kind == events.MoveZone && o.Owner == 0 && o.Face().Name == "Defiler of Flesh":
			defilers[e.Obj] = e.To == state.ZBattlefield
		case e.Kind == events.PutOnStack && e.From == state.ZExile && o.Face().Name == card:
			for _, on := range defilers {
				if on {
					return true
				}
			}
		}
	}
	return false
}
