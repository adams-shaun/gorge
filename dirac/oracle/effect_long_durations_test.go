package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Effects whose grant outlives the turn, played decision by decision against
// gorge: End-Blaze Epiphany's may-play of the chosen exiled card until the end
// of its controller's next turn, and The Wondrous Wasp's loss of all abilities
// for as long as it stays on the battlefield. TestStdSetReprosLockstep replays
// the two repros; TestEffectLongDurationsReprosReach requires each recorded
// game to reach the grant, since a lockstep entry alone would pass a script
// that never cast the card.
//
// The games are recorded by TestEffectLongDurationsRecord
// (EFFECT_LONG_DURATIONS_RECORD=1) on the kernel table built from the decks:
// both seats prefer the case's cards and otherwise pick at random from the
// seed's own stream.
type effectLongCase struct {
	name   string
	prefer []string
	decks  [2][]string
	// surface is the option surface both sides offer: End-Blaze Epiphany's
	// {X} cost is priced only on the full one.
	surface string
	// reached reads the finished gorge log for the grant's observable effect.
	reached func(g *GorgeSide) bool
}

func effectLongCases() []effectLongCase {
	return []effectLongCase{
		{
			name:   "end_blaze_chosen_card_played_next_turn",
			prefer: []string{"End-Blaze Epiphany", "Grizzly Bears"},
			decks: [2][]string{
				append(deckOf(8, "End-Blaze Epiphany"), deckOf(16, "Mountain")...),
				append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
			},
			surface: "full",
			reached: endBlazePlayedNextTurn,
		},
		{
			name:   "wondrous_wasp_target_loses_abilities",
			prefer: []string{"The Wondrous Wasp", "Llanowar Elves"},
			decks: [2][]string{
				append(deckOf(8, "The Wondrous Wasp"), deckOf(16, "Island")...),
				append(deckOf(8, "Llanowar Elves"), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: waspTapsOpposingCreature,
		},
	}
}

// endBlazePlayedNextTurn: cards were exiled from a library, and a later turn
// moved one of them from exile onto the stack or the battlefield.
func endBlazePlayedNextTurn(g *GorgeSide) bool {
	turn, exiledOn := 0, 0
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			turn++
		case e.Kind == events.MoveZone && e.From == state.ZLibrary && e.To == state.ZExile:
			if exiledOn == 0 {
				exiledOn = turn
			}
		case exiledOn != 0 && turn > exiledOn && e.Kind == events.MoveZone && e.From == state.ZExile &&
			(e.To == state.ZStack || e.To == state.ZBattlefield):
			return true
		}
	}
	return false
}

// waspTapsOpposingCreature: a Wasp entered and a creature its controller does
// not control was tapped after it (the enters trigger's tap, then its loss of
// abilities, which the later turns play out).
func waspTapsOpposingCreature(g *GorgeSide) bool {
	var wasp state.ObjID
	for _, e := range g.E.L.Events {
		if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "The Wondrous Wasp" {
				wasp = e.Obj
			}
		}
		if wasp != 0 && e.Kind == events.Tap && e.Obj != wasp {
			t, w := g.E.G.Obj(e.Obj), g.E.G.Obj(wasp)
			if t != nil && w != nil && t.Face() != nil && t.Face().Name == "Llanowar Elves" && t.Controller != w.Controller {
				return true
			}
		}
	}
	return false
}

// effectLongPolicy casts the case's cards first and otherwise plays a land
// or picks at random from the seed's own stream.
func effectLongPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, want := range c.prefer {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

func effectLongPlay(t *testing.T, reg *cards.Registry, c effectLongCase, r *Repro, set cardset.Set, pol Policy) (Result, bool) {
	t.Helper()
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	} else {
		rc.MaxDecisions = 2000
	}
	var reached bool
	rc.Finish = func(g *GorgeSide) {
		reached = c.reached(g)
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), reached
}

// TestEffectLongDurationsReprosReach replays the recorded games: no
// divergence, every scripted action offered, the grant reached.
func TestEffectLongDurationsReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range effectLongCases() {
		t.Run(c.name, func(t *testing.T) {
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
				t.Fatalf("the recorded game never reaches the grant of %s", c.name)
			}
		})
	}
}

// TestEffectLongDurationsRecord plays the cases' policies over seeds until
// one reaches the grant, and writes the repro. It runs only with
// EFFECT_LONG_DURATIONS_RECORD=1; the games it writes are the committed repros.
func TestEffectLongDurationsRecord(t *testing.T) {
	if os.Getenv("EFFECT_LONG_DURATIONS_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range effectLongCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, effectLongPolicy(m, c, seed))
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
			t.Fatalf("no seed reached the grant of %s", c.name)
		})
	}
}
