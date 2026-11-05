package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestXSpellPoolLockstep: Khaaaaaaaaaaaannn! ({X}{R}) cast with the largest X
// that a board of seven lands and nineteen Treasures pays, on the full surface
// (the player floats the planner's witness before the cast). The plan over
// those twenty-six sources reaches gorge's node limit (pay.Search,
// MaxPaymentPlanSearchNodes) in phase 2, where a sacrificed source's cost is
// rank key 1, and gorge takes the best plan it has seen instead of the key-8
// least: thirteen Treasures make {G} and six {R}, and the Godless Shrine makes
// {W}. The kernel replays that search (mana_planlimit.go) so the floating pool
// holds the same colours. The recorded game is pair 613 of the std-trk sweep:
// the random play of a fixture of ordinary size does not reach this board.
func TestXSpellPoolLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-trk")
	r, err := ReadRepro("testdata/repro/x_spell_pool_treasure_plan_limit.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Surface == "autopay" {
		t.Fatalf("the recorded game is on the %s surface; the divergence is the full one", r.Surface)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		var es []cardset.Entry
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	floated := 0
	rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
		for p := range ref.E.G.Players {
			floated = max(floated, int(ref.E.G.Players[p].Pool.Total()))
		}
		return nil
	}
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	CloseSide(k)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.StallUnmodelled {
		t.Fatalf("dirac stopped at a rule it does not model: %s", res.Stall)
	}
	if floated < 26 {
		t.Fatalf("the game floated at most %d mana: it never reaches the board the pin is about", floated)
	}
}
