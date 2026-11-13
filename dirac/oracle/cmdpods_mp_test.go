//go:build dirac_mp

package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cmdPodFeatures counts the Commander rules a pod game exercised, read off
// gorge's finished game: CR 903.9a command-zone asks, the most command-zone
// casts of one commander (CR 903.8: a second cast pays tax), the most
// opponents whose commanders dealt one player combat damage (CR 903.10a),
// and seats that left the game with a commander of theirs on the
// battlefield (CR 800.4a).
type cmdPodFeatures struct {
	asks, maxCasts, maxDamagers, leftInPlay int
}

func countCmdPodFeatures(g *GorgeSide) cmdPodFeatures {
	var f cmdPodFeatures
	gm := g.E.G
	owner := map[state.ObjID]int{}
	var dense []int // the match-wide commander index's owner
	for p := range gm.Players {
		for _, c := range gm.Players[p].Commanders {
			owner[c] = p
			dense = append(dense, p)
		}
		for _, n := range gm.Players[p].CmdCasts {
			f.maxCasts = max(f.maxCasts, int(n))
		}
		seen := map[int]bool{}
		for k, d := range gm.Players[p].CmdDamage {
			if d > 0 && k < len(dense) {
				seen[dense[k]] = true
			}
		}
		f.maxDamagers = max(f.maxDamagers, len(seen))
	}
	zone := map[state.ObjID]state.Zone{}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch e.Kind {
		case events.DecisionAsk:
			if e.Text == "commander_zone" {
				f.asks++
			}
		case events.MoveZone:
			if _, ok := owner[e.Obj]; ok {
				zone[e.Obj] = e.To
			}
		case events.PlayerLost:
			for c, p := range owner {
				if p == int(e.Player) && zone[c] == state.ZBattlefield {
					f.leftInPlay++
					break
				}
			}
		}
	}
	return f
}

// TestFourSeatCommanderMiniPodsLockstep plays four-seat pods of the cmd5
// Commander decks, each cut to its commander and 39 more cards
// (MiniCommanderDeck), dirac against gorge, as kfuzz -kernel cmd5 -seats 4
// -cmd-mini 40 does: no pod diverges, and across them a commander is asked
// back to the command zone, cast a second time (with tax), and still on the
// battlefield when its owner leaves the game, and one player takes combat
// damage from the commanders of all three opponents (pod 194; CR 903.10a
// keeps a tally per commander). Pods 8 and 32 carry a seat-2 permanent whose
// mana ability a seat-2 static grants (Rishkar, Peema Renegade): the grant
// walk read only seats 0 and 1.
func TestFourSeatCommanderMiniPodsLockstep(t *testing.T) {
	c, names := cmd5MiniCorpus(t, 40)
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	pool := make([]PoolDeck, len(names))
	for i, n := range names {
		pool[i] = PoolDeck{Name: n}
	}
	var sum cmdPodFeatures
	for _, i := range []int{8, 32, 194, 0, 1, 2, 3, 4, 5, 6, 7} {
		p := PodAt(pool, i, 1, 4)
		m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		rc := RunConfig{MaxDecisions: 20000, MaxTurns: 100}
		var f cmdPodFeatures
		rc.Finish = func(g *GorgeSide) { f = countCmdPodFeatures(g) }
		k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		res := Lockstep(m, p.Seed, k, AffordableTargetPolicy(RandomPolicy(p.Seed)), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("pod %d (%v): divergence at %d: %s %s %v", i, p.Decks(), res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		t.Logf("pod %d: %d decisions, stall %q, %+v", i, res.Decisions, res.Stall, f)
		sum.asks += f.asks
		sum.maxCasts = max(sum.maxCasts, f.maxCasts)
		sum.maxDamagers = max(sum.maxDamagers, f.maxDamagers)
		sum.leftInPlay += f.leftInPlay
	}
	if sum.asks == 0 || sum.maxCasts < 2 || sum.maxDamagers < 3 || sum.leftInPlay == 0 {
		t.Fatalf("Commander rules not all exercised: %+v", sum)
	}
}

// TestFourSeatOnslaughtPastEightTargets: pod 100's Monstrous Onslaught
// ("X damage divided as you choose among any number of target creatures")
// is cast into a four-seat board with nine or more creatures picked, past
// the default class's eight-object slot (maxXTgt). The four-seat class holds
// sixteen, so the pod plays to its end in lockstep instead of stalling
// StallXTgt at the ninth pick.
func TestFourSeatOnslaughtPastEightTargets(t *testing.T) {
	c, names := cmd5MiniCorpus(t, 40)
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	pool := make([]PoolDeck, len(names))
	for i, n := range names {
		pool[i] = PoolDeck{Name: n}
	}
	p := PodAt(pool, 100, 1, 4)
	m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
	if err != nil {
		t.Fatal(err)
	}
	rc := RunConfig{MaxDecisions: 20000, MaxTurns: 100}
	most := 0
	rc.Finish = func(g *GorgeSide) {
		// fold the TargetsChosen shapes per stack object: 0 replaces the
		// object targets, 2 appends them (events.foldTargetsChosen)
		n := map[state.ObjID]int{}
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind != events.TargetsChosen || e.Text == events.SubTargetNotice {
				continue
			}
			switch e.Amount {
			case 0:
				n[e.Obj] = len(e.IDs)
			case 2:
				n[e.Obj] += len(e.IDs)
			}
			most = max(most, n[e.Obj])
		}
	}
	k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, p.Seed, k, AffordableTargetPolicy(RandomPolicy(p.Seed)), rc)
	CloseSide(k)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Stall != "" {
		t.Fatalf("stalled %q after %d decisions", res.Stall, res.Decisions)
	}
	if most < 9 {
		t.Fatalf("the most targets one spell took is %d; want the ninth pick that overflowed eight", most)
	}
	t.Logf("%d decisions; one spell took %d targets", res.Decisions, most)
}
