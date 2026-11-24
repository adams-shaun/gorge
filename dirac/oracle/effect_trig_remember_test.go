package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// An Effect's trigger bound to the set RememberObjects$ names, played decision
// by decision against gorge: The Eagles Are Coming! (a Bird at the next upkeep
// for each creature the spell returned) and T'Pring, Never Parted (a card drawn
// when either fighter dies this turn).
// TestStdSetReprosLockstep replays the two repros; TestEffectTrigRememberReprosReach
// requires each recorded game to reach the trigger's observable effect, since a
// lockstep entry alone would pass a script that never cast the card.
//
// The games are recorded by TestEffectTrigRememberRecord
// (EFFECT_TRIG_REMEMBER_RECORD=1) on the kernel table built from the decks.
// Mysterio, Master of Illusion's recordings are gone because the card is now
// refused by kgen: gorge's source-leave delayed trigger fires (CR 603.7b) and
// dirac still lowers the old emblem shape (lt-low-effect-trig-remember), so its
// refused entry (std-spm, autopay) is the game that shows the difference.
func effectTrigRememberCases() []effectLongCase {
	return []effectLongCase{
		{
			name:   "effect_trig_remember_eagles_bird_per_returned_creature",
			prefer: []string{"The Eagles Are Coming!", "Savannah Lions"},
			decks: [2][]string{
				append(append(deckOf(6, "The Eagles Are Coming!"), deckOf(6, "Savannah Lions")...), deckOf(12, "Plains")...),
				append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: eaglesBirdAtUpkeep,
		},
		{
			name:   "effect_trig_remember_tpring_draws_when_a_fighter_dies",
			prefer: []string{"T'Pring, Never Parted", "Grizzly Bears"},
			decks: [2][]string{
				append(append(deckOf(6, "T'Pring, Never Parted"), deckOf(6, "Grizzly Bears")...), append(deckOf(6, "Forest"), deckOf(6, "Island")...)...),
				append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: tpringDrawsOnDeath,
		},
	}
}

// eaglesBirdAtUpkeep: a creature of seat 0's returned from the battlefield to
// its hand (the spell), and a Bird Soldier token was minted at a later step.
func eaglesBirdAtUpkeep(g *GorgeSide) bool {
	returned := false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZHand && sideObjName(g, e.Obj) == "Savannah Lions":
			returned = true
		case returned && e.Kind == events.TokenCreate && strings.Contains(e.Text, "bird"):
			return true
		}
	}
	return false
}

// tpringDrawsOnDeath: after T'Pring entered, a Grizzly Bears died and seat 0
// drew a card before the turn ended (outside the draw step, the trigger's).
func tpringDrawsOnDeath(g *GorgeSide) bool {
	entered, died := false, false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			entered, died = false, false
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && sideObjName(g, e.Obj) == "T'Pring, Never Parted":
			entered = true
		case entered && e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard && sideObjName(g, e.Obj) == "Grizzly Bears":
			died = true
		case died && e.Kind == events.Draw && e.Player == 0:
			return true
		}
	}
	return false
}

func TestEffectTrigRememberReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range effectTrigRememberCases() {
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
				t.Fatalf("the recorded game never reaches %s", c.name)
			}
		})
	}
}

// TestEffectTrigRememberRecord plays each case's policy over seeds until one
// reaches the effect and writes the repro. It runs only with
// EFFECT_TRIG_REMEMBER_RECORD=1.
func TestEffectTrigRememberRecord(t *testing.T) {
	if os.Getenv("EFFECT_TRIG_REMEMBER_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range effectTrigRememberCases() {
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
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
