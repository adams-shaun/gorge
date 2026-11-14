package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Tromokratis: S:Mode$ MinMaxBlocker | ValidCard$ Card.Self | Min$ All
// (CR 509.1b, kop.MinBlockersAll: "can't be blocked unless all creatures
// defending player controls block it") against small creatures, some
// tapped from their own attacks, and burn. gorge's blockPairScopeFor offers
// a Tromokratis block only while every creature the defender controls can
// block it, and ValidateMinMaxBlockers admits only the whole team; the
// kernel's minBlockers floors it at the attacked player's creature count.
var ktromokratisDecks = [2][]cardset.Entry{
	{
		{Name: "Tromokratis", Count: 10}, {Name: "Island", Count: 14}, {Name: "Forest", Count: 6},
		{Name: "Llanowar Elves", Count: 6}, {Name: "Grizzly Bears", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 6},
	},
}

// TestTromokratisMinAllLockstep plays the pool against gorge with
// castFirstPolicy and fails on any divergence. Counted per Tromokratis
// attack: the most creatures seen blocking it and the defender's creature
// count when it was first seen blocked; a blocked attack must be blocked by
// all of them (all or none).
func TestTromokratisMinAllLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := p3Setup(t, "tromokratis", ktromokratisDecks)
	type key struct {
		seed uint64
		turn int32
		obj  state.ObjID
	}
	type atk struct{ blockers, team int }
	attacks := map[key]*atk{}
	var seed uint64
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				o := g.Obj(id)
				if o == nil || !o.IsAttacking || pkFaceName(o) != "Tromokratis" {
					continue
				}
				kk := key{seed, g.Turn, id}
				a := attacks[kk]
				if a == nil {
					a = &atk{}
					attacks[kk] = a
				}
				if n := len(o.BlockedBy); n > a.blockers {
					if a.blockers == 0 {
						a.team = tmCreatures(ref, state.PlayerID(1-p))
					}
					a.blockers = n
				}
			}
		}
		return nil
	}
	const seeds = 120
	clean := 0
	for seed = 1; seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"tromokratis-a", "tromokratis-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			continue
		}
		clean++
	}
	blocked, partial := 0, 0
	for kk, a := range attacks {
		if a.blockers == 0 {
			continue
		}
		blocked++
		if a.blockers != a.team {
			partial++
			t.Errorf("seed %d turn %d: Tromokratis blocked by %d of the defender's %d creatures", kk.seed, kk.turn, a.blockers, a.team)
		}
	}
	t.Logf("%d seeds, %d clean: %d Tromokratis attacks, %d blocked by the whole team, %d partially", seeds, clean, len(attacks), blocked-partial, partial)
	if len(attacks) < 30 || blocked < 5 {
		t.Errorf("too little coverage: %d Tromokratis attacks, %d blocked", len(attacks), blocked)
	}
}

// tmCreatures counts the creatures player p controls (gorge
// combat.DefenderCreatureCount's reading: the layer-derived creature test).
func tmCreatures(ref *GorgeSide, p state.PlayerID) int {
	n := 0
	for _, id := range ref.E.G.Zone(state.ZBattlefield, p) {
		if ref.E.IsCreature(id) {
			n++
		}
	}
	return n
}
