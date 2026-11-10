package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// An Effect whose static is "can't cast spells" (Bilbo's Gambit with its gift
// promised; Calamity's Wake), played decision by decision against gorge.
// TestStdSetReprosLockstep replays the repros; TestCantBeCastReprosReach
// requires each recorded game to reach the card, since a lockstep entry alone
// would pass a script that never cast it.
//
// The games are recorded by TestCantBeCastRecord (CANT_BE_CAST_RECORD=1) on
// the kernel table built from the decks. Seat 1 holds only Lightning Bolts, and
// seat 0 casts its card only while seat 1 still holds a Bolt and an untapped
// land, so the emblem withholds a cast the seat would otherwise be offered.
func cantBeCastCases() []effectLongCase {
	bolts := append(deckOf(8, "Lightning Bolt"), deckOf(16, "Mountain")...)
	return []effectLongCase{
		{
			name:    "bilbos_gambit_gift_promised_players_cannot_cast",
			prefer:  []string{"Bilbo's Gambit"},
			decks:   [2][]string{append(deckOf(8, "Bilbo's Gambit"), deckOf(16, "Plains")...), bolts},
			surface: "autopay",
			reached: func(g *GorgeSide) bool {
				return movedFrom("Bilbo's Gambit", state.ZStack, state.ZGraveyard)(g) && tokenMinted(g)
			},
		},
		{
			name:    "calamitys_wake_players_cannot_cast_noncreature_spells",
			prefer:  []string{"Calamity's Wake"},
			decks:   [2][]string{append(deckOf(8, "Calamity's Wake"), deckOf(16, "Plains")...), bolts},
			surface: "autopay",
			reached: movedFrom("Calamity's Wake", state.ZStack, state.ZExile),
		},
	}
}

// tokenMinted: a token was created (only the gift can: no card of the
// recorded decks makes one).
func tokenMinted(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind == events.TokenCreate {
			return true
		}
	}
	return false
}

// boltHeld: seat 1 holds a Lightning Bolt and has an untapped land.
func boltHeld(g *GorgeSide) bool {
	if g == nil {
		return false
	}
	bolt, land := false, false
	for _, id := range g.E.G.Zone(state.ZHand, 1) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Lightning Bolt" {
			bolt = true
		}
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, 1) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mountain" && !o.Tapped {
			land = true
		}
	}
	return bolt && land
}

// cantBeCastPolicy is effectLongPolicy, except that the case's card (prefer[0])
// is not cast while seat 1 cannot cast a Bolt, and the gift is promised when
// asked.
func cantBeCastPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority || boltHeld(p.G) {
			return inner.Pick(p)
		}
		d := *p.D
		d.Options = nil
		for _, o := range p.D.Options {
			if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == c.prefer[0] {
				continue
			}
			d.Options = append(d.Options, o)
		}
		return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

// TestCantBeCastReprosReach replays the recorded games: no divergence, every
// scripted action offered, the card reached.
func TestCantBeCastReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range cantBeCastCases() {
		t.Run(c.name, func(t *testing.T) {
			cantBeCastReplay(t, reg, c)
		})
	}
}

func cantBeCastReplay(t *testing.T, reg *cards.Registry, c effectLongCase) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatalf("the recorded game never reaches %s", c.name)
	}
}

// TestCantBeCastRecord plays the cases' policies over seeds until one reaches
// the card, and writes the repro. It runs only with CANT_BE_CAST_RECORD=1.
func TestCantBeCastRecord(t *testing.T) {
	if os.Getenv("CANT_BE_CAST_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range cantBeCastCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, cantBeCastPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
