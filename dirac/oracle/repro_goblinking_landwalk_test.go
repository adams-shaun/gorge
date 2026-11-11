package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Goblin King's "Other Goblins get +1/+1 and have mountainwalk" is a
// Continuous static granting AddKeyword$ Landwalk:Mountain (CR 702.14,
// 613.1f): kgen lowers it as kop.Static.Landwalk, and dirac's block legality
// (landwalkGranted) reads it like a printed CardDef.Landwalk, gorge's
// combat.LandwalkEvades over the derived keywords.
//
// Both games: the first player casts Goblin King, then a Goblin Piker (a
// Goblin that is not the King), and attacks with the Piker alone; the second
// player never attacks, so their Grizzly Bears are untapped on the first
// player's turn.
//
//   - Against a Mountain the Bears cannot block the Piker: the block menu has
//     no blocker for it (the Piker has mountainwalk).
//   - With no Mountain (Forests only) the Bears can, and block it.
//
// The games are recorded by TestReproGoblinKingLandwalkRecord
// (GOBLIN_KING_RECORD=1) and replayed, with a requirement that they reach the
// attack under each board, by TestReproGoblinKingLandwalkReach.
type goblinKingCase struct {
	effectLongCase
	mountain bool // the second player controls a Mountain: the Piker is unblockable
}

func goblinKingCases() []goblinKingCase {
	prefer := []string{"Goblin King", "Goblin Piker", "Grizzly Bears"}
	mine := append(append(deckOf(4, "Goblin King"), deckOf(8, "Goblin Piker")...), deckOf(12, "Mountain")...)
	return []goblinKingCase{
		{effectLongCase: effectLongCase{
			name:   "repro_goblin_king_mountainwalk_unblockable_over_mountain",
			prefer: prefer,
			decks: [2][]string{
				mine,
				append(append(deckOf(8, "Mountain"), deckOf(8, "Forest")...), deckOf(8, "Grizzly Bears")...),
			},
			surface: "autopay",
			reached: func(g *GorgeSide) bool { return goblinKingAttack(g, true) },
		}, mountain: true},
		{effectLongCase: effectLongCase{
			name:   "repro_goblin_king_mountainwalk_blockable_without_mountain",
			prefer: prefer,
			decks: [2][]string{
				mine,
				append(deckOf(16, "Forest"), deckOf(8, "Grizzly Bears")...),
			},
			surface: "autopay",
			reached: func(g *GorgeSide) bool { return goblinKingAttack(g, false) },
		}},
	}
}

// goblinKingAttack reads the finished gorge log: with Goblin King, the second
// player's Grizzly Bears (and, when mountain, a Mountain) on the battlefield,
// the first player declares a Goblin Piker as an attacker (a DeclareAttackers
// event's Player is the defender); the second player never attacked (so the Bears were untapped). Then, over a Mountain, no
// block was declared on the Piker; without one, the Bears blocked it.
func goblinKingAttack(g *GorgeSide, mountain bool) bool {
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	king, bears, mtn, set := false, false, false, false
	var piker state.ObjID
	blocked := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield:
			o := g.E.G.Obj(e.Obj)
			switch {
			case o == nil:
			case name(e.Obj) == "Goblin King" && o.Controller == 0:
				king = true
			case name(e.Obj) == "Grizzly Bears" && o.Controller == 1:
				bears = true
			case name(e.Obj) == "Mountain" && o.Controller == 1:
				mtn = true
			}
		case e.Kind == events.DeclareAttackers && e.Player == 0 && len(e.IDs) > 0:
			return false // an attack on the first player (Player is the defender): the Bears would be tapped on their turn
		case e.Kind == events.DeclareAttackers && e.Player == 1 && !set:
			for _, id := range e.IDs {
				if name(id) == "Goblin Piker" && king && bears && mtn == mountain {
					piker, set = id, true
				}
			}
		case e.Kind == events.DeclareBlockers && set:
			for _, pr := range e.Pairs {
				if pr[0] == piker && name(pr[1]) == "Grizzly Bears" {
					blocked = true
				}
			}
		}
	}
	return set && blocked == !mountain
}

// goblinKingPolicy casts the King, then the Piker, plays lands, attacks with
// the Piker only, never attacks with the second player's creatures, and blocks
// the Piker whenever it may; anything else is the seed's random pick.
func goblinKingPolicy(m *Matchup, c goblinKingCase, seed uint64) Policy {
	base := effectLongPolicy(m, c.effectLongCase, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		switch p.D.Kind {
		case DAttackers:
			for _, o := range p.D.Options {
				if o.Kind() == ActAttack && p.D.Player == 0 && m.InstanceCard(int(o.Src())) == "Goblin Piker" {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() != ActAttack {
					return o
				}
			}
		case DBlockers:
			for _, o := range p.D.Options {
				if o.Kind() == ActBlock && m.InstanceCard(int(o.Tgt())) == "Goblin Piker" {
					return o
				}
			}
		}
		return base.Pick(p)
	})
}

// TestReproGoblinKingLandwalkReach replays the recorded games: no divergence,
// every scripted action offered, and the Piker attacks under the board each
// game names with the block outcome it implies.
func TestReproGoblinKingLandwalkReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range goblinKingCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c.effectLongCase, r, printedDelveSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches the Goblin Piker's attack (mountain=%v) with the block outcome it implies", c.mountain)
			}
		})
	}
}

// TestReproGoblinKingLandwalkRecord plays each policy over seeds until one
// reaches the attack, and writes the repro. It runs only with
// GOBLIN_KING_RECORD=1; the games it writes are the committed repros.
func TestReproGoblinKingLandwalkRecord(t *testing.T) {
	if os.Getenv("GOBLIN_KING_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range goblinKingCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"},
					Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				set := reproSet(t, "custom", r)
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, goblinKingPolicy(m, c, seed))
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
			t.Fatal("no seed reached the Goblin Piker's attack")
		})
	}
}
