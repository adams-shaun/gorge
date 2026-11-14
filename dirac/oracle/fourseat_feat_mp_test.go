//go:build dirac_mp

package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestFeatFourSeatVanilla is TestFeatVanilla at three and four seats: the
// real kernel in lockstep with gorge on vanilla pods with FeatCheck on, so
// at every decision the kernel's encoded network inputs (the four relative
// seat blocks, the bags and entity lists over every other seat, the option
// rows' relative-seat one-hot) equal policynet's encoding of gorge's view.
// Random and heuristic policies, so attacks at each defender are reached.
func TestFeatFourSeatVanilla(t *testing.T) {
	t.Parallel()

	c, _ := testCorpus(t)
	pool, err := FeatPool(c.Reg, vanilla.Table, nil)
	if err != nil {
		t.Fatal(err)
	}
	mk := RealKernel(vanilla.Table, nil, Config{})
	n := len(vanilla.Decks)
	var total FeatStats
	for g := 0; g < 8; g++ {
		seats := 3 + g%2
		labels := make([]string, seats)
		decks := make([][]kop.CardID, seats)
		for i := range decks {
			d := (g + i) % n
			labels[i], decks[i] = fmt.Sprint("vanilla", d), vanilla.Decks[d]
		}
		seed := uint64(4200 + g)
		m, err := KernelMatchupN(c.Reg, vanilla.Table, labels, decks, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := mk(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var st FeatStats
		pol := RandomPolicy(seed)
		if g%4 >= 2 {
			pol = HeuristicPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 4000, MaxTurns: 40, Check: FeatCheck(pool, &st)})
		CloseSide(k)
		total.Decisions += st.Decisions
		total.Options += st.Options
		if d := res.Div; d != nil {
			t.Errorf("game %d (%d seats): after %d compared decisions: %s %s: %s", g, seats, st.Decisions, d.What, d.Field, d.Detail)
		}
	}
	if total.Decisions < 500 {
		t.Fatalf("compared only %d decisions", total.Decisions)
	}
	t.Logf("compared %d decisions, %d options", total.Decisions, total.Options)
}

// TestFeatFourSeatFDN is TestFeatFourSeatVanilla over four-seat FDN pods
// (two random pairings' decks each), where targets, the stack and
// noncreature permanents reach the relative-seat encodings. A pod the
// tables cannot build is skipped; a rules divergence ends a pod's
// comparison and is logged (the lockstep gate owns those), a feature
// mismatch fails.
func TestFeatFourSeatFDN(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	var total FeatStats
	pods := 0
	ps := Pairings(pool, 16, 13)
	for i := 0; i+1 < len(ps) && pods < 6; i += 2 {
		names := []string{ps[i].A, ps[i].B, ps[i+1].A, ps[i+1].B}
		m, tbl, err := SetMatchupN(rawSet("fdn"), c, names, 0)
		if err != nil {
			continue
		}
		fp, err := FeatPool(c.Reg, tbl, nil)
		if err != nil {
			t.Fatal(err)
		}
		seed := ps[i].Seed
		k, err := SetSide(rawSet("fdn"), tbl, Config{})(m, seed)
		if err != nil {
			continue
		}
		var st FeatStats
		pol := RandomPolicy(seed)
		if pods%2 == 1 {
			pol = HeuristicPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 4000, MaxTurns: 40, Check: FeatCheck(fp, &st)})
		CloseSide(k)
		pods++
		total.Decisions += st.Decisions
		total.Options += st.Options
		if d := res.Div; d != nil {
			if d.What == "features" || d.Field == "panic" {
				t.Errorf("pod %v: after %d compared decisions: %s %s: %s", names, st.Decisions, d.What, d.Field, d.Detail)
			} else {
				t.Logf("pod %v: rules divergence after %d compared decisions: %s %s", names, st.Decisions, d.What, d.Field)
			}
		}
	}
	if pods == 0 || total.Decisions < 200 {
		t.Fatalf("%d pods, %d compared decisions", pods, total.Decisions)
	}
	t.Logf("%d pods: compared %d decisions, %d options", pods, total.Decisions, total.Options)
}

// TestFeatFourSeatHundred is TestFeatFourSeatVanilla at four seats of
// 100-card decks (the vanilla decks repeated): every position holds 400
// objects or more, so every object of seats 2 and 3 is past 255 and the
// encoders must name objects at the kernel's object width to match
// gorge's encoding of the same position. A rules divergence ends a game's
// comparison and is logged (the lockstep gate owns those); a feature
// mismatch or a panic fails. Not parallel: CloseSide releases the vanilla
// table, which TestFeatFourSeatVanilla's games play too.
func TestFeatFourSeatHundred(t *testing.T) {
	c, _ := testCorpus(t)
	pool, err := FeatPool(c.Reg, vanilla.Table, nil)
	if err != nil {
		t.Fatal(err)
	}
	mk := RealKernel(vanilla.Table, nil, Config{})
	n := len(vanilla.Decks)
	var total FeatStats
	for g := 0; g < 2; g++ {
		labels := make([]string, 4)
		decks := make([][]kop.CardID, 4)
		for i := range decks {
			d := (g + i) % n
			labels[i] = fmt.Sprint("vanilla", d, "x100")
			for len(decks[i]) < 100 {
				decks[i] = append(decks[i], vanilla.Decks[d][:min(len(vanilla.Decks[d]), 100-len(decks[i]))]...)
			}
		}
		seed := uint64(4300 + g)
		m, err := KernelMatchupN(c.Reg, vanilla.Table, labels, decks, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := mk(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var st FeatStats
		pol := RandomPolicy(seed)
		if g == 1 {
			pol = HeuristicPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 4000, MaxTurns: 40, Check: FeatCheck(pool, &st)})
		CloseSide(k)
		total.Decisions += st.Decisions
		total.Options += st.Options
		if d := res.Div; d != nil {
			if d.What == "features" || d.Field == "panic" {
				t.Errorf("game %d: after %d compared decisions: %s %s: %s", g, st.Decisions, d.What, d.Field, d.Detail)
			} else {
				t.Logf("game %d: rules divergence after %d compared decisions: %s %s: %s", g, st.Decisions, d.What, d.Field, d.Detail)
			}
		}
	}
	if total.Decisions < 1000 {
		t.Fatalf("compared only %d decisions", total.Decisions)
	}
	t.Logf("compared %d decisions, %d options", total.Decisions, total.Options)
}
