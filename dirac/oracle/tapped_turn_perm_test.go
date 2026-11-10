package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Captain America, Living Legend's Taps FirstTime$ trigger: a creature you
// control tapped for the first time this turn is untapped. The game plays
// decision by decision against gorge; TestStdSetReprosLockstep replays it.
//
// It is recorded by TestTappedTurnPermRecord (TAPPED_TURN_PERM_RECORD=1) on
// the table built from its own decks, with effectLongPolicy.
const tappedTurnPermCard = "Captain America, Living Legend"

func tappedTurnPermCase() effectLongCase {
	return effectLongCase{
		name:   "captain_america_first_tap_untaps",
		prefer: []string{tappedTurnPermCard, "Wind Drake"},
		decks: [2][]string{
			append(deckOf(4, tappedTurnPermCard), append(deckOf(12, "Wind Drake"), append(deckOf(10, "Plains"), deckOf(10, "Island")...)...)...),
			append(deckOf(12, "Grizzly Bears"), deckOf(24, "Forest")...),
		},
		surface: "autopay",
		reached: tappedTurnPermReached,
	}
}

// tappedTurnPermPolicy is effectLongPolicy with every attack declared: the
// trigger needs a creature to attack (or otherwise tap) on Captain America's
// controller's turn, which the random tail does not reliably do.
func tappedTurnPermPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DAttackers && p.D.Player == 0 {
			for _, o := range p.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		}
		if p.D.Kind == DBlockers && p.D.Player == 0 {
			for _, o := range p.D.Options {
				if o.Kind() == ActBlock {
					return o
				}
			}
		}
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

// tappedTurnPermReached: Captain America was on the battlefield, a creature
// then became tapped on its controller's turn, and the same permanent became
// untapped later in that turn (the trigger's effect).
func tappedTurnPermReached(g *GorgeSide) bool {
	turn := 0
	ca := false
	tappedOn := make(map[state.ObjID]int)
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			turn++
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && ownCardName(g.E, e.Obj) == tappedTurnPermCard:
			ca = true
		case ca && e.Kind == events.Tap:
			if _, ok := tappedOn[e.Obj]; !ok {
				tappedOn[e.Obj] = turn
			}
		case ca && e.Kind == events.Untap:
			if at, ok := tappedOn[e.Obj]; ok && at == turn {
				return true
			}
		}
	}
	return false
}

// TestTappedTurnPermRecord plays the policy over seeds until one reaches the
// trigger, and writes the repro. It runs only with TAPPED_TURN_PERM_RECORD=1;
// the game it writes is the committed repro.
func TestTappedTurnPermRecord(t *testing.T) {
	if os.Getenv("TAPPED_TURN_PERM_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range []effectLongCase{tappedTurnPermCase(), taleTappedTurnCase()} {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, tappedTurnPermPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %v\nker %v\n%s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
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
			t.Fatalf("no seed reached the trigger's effect of %s", c.name)
		})
	}
}

// TestTappedTurnPermReprosReach replays the recorded game and requires it to
// reach the trigger's untap, since a lockstep entry alone would pass a script
// that never tapped a creature under Captain America.
func TestTappedTurnPermReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range []effectLongCase{tappedTurnPermCase(), taleTappedTurnCase()} {
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
				t.Fatalf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatal("the recorded game never reached the trigger's effect")
			}
		})
	}
}

// Tale of Katara and Toph grants its creatures' self Taps FirstTime$ trigger:
// the same per-slot mark, keyed by the tapped creature, gives the +1/+1
// counter once. It is the second card the Taps FirstTime$ lowering moves.
const taleTappedTurnCard = "Tale of Katara and Toph"

func taleTappedTurnCase() effectLongCase {
	return effectLongCase{
		name:   "tale_of_katara_and_toph_first_tap_counter",
		prefer: []string{taleTappedTurnCard, "Grizzly Bears"},
		decks: [2][]string{
			append(deckOf(4, taleTappedTurnCard), append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...)...),
			append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
		},
		surface: "autopay",
		reached: taleTappedTurnReached,
	}
}

// taleTappedTurnReached: the granted trigger put a +1/+1 counter on a
// creature (the first tap's reward).
func taleTappedTurnReached(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind == events.CounterChange && e.Amount > 0 && e.Counter == "P1P1" && ownCardName(g.E, e.Obj) != "" {
			return true
		}
	}
	return false
}
