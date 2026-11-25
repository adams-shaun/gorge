package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestFeatFourSeatVanilla is TestFeatVanilla at three and four seats: the
// real kernel in lockstep with gorge on vanilla pods with FeatCheck on, so
// at every decision the kernel's encoded network inputs (the four relative
// seat blocks, the bags and entity lists over every other seat, the option
// rows' relative-seat one-hot) equal policynet's encoding of gorge's view.
// Random and heuristic policies, so attacks at each defender are reached.
// Every game is compared under FeaturesEntity and under FeaturesPod.
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
	for gi := 0; gi < 16; gi++ {
		g, fs := gi%8, policynet.FeaturesEntity
		if gi >= 8 {
			fs = policynet.FeaturesPod
		}
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
		res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 4000, MaxTurns: 40, Check: FeatCheckFS(pool, &st, fs)})
		// No CloseSide: the side plays the process-wide vanilla.Table, which the
		// parallel TestFeatVanilla plays too, and ReleaseTable invalidates every
		// State playing the table (leaving its registry slot nil).
		total.Decisions += st.Decisions
		total.Options += st.Options
		if d := res.Div; d != nil {
			t.Errorf("game %d (%d seats, %s): after %d compared decisions: %s %s: %s", g, seats, fs, st.Decisions, d.What, d.Field, d.Detail)
		}
	}
	if total.Decisions < 1000 {
		t.Fatalf("compared only %d decisions", total.Decisions)
	}
	t.Logf("compared %d decisions, %d options", total.Decisions, total.Options)
}

// TestFeatFourSeatFDN is TestFeatFourSeatVanilla over four-seat FDN pods
// (two random pairings' decks each), where targets, the stack and
// noncreature permanents reach the relative-seat encodings. A pod the
// tables cannot build is skipped; a rules divergence ends a pod's
// comparison and is logged (the lockstep gate owns those), a feature
// mismatch fails. Each pod is compared under FeaturesEntity and again under
// FeaturesPod, where the entity rows' controller and attacked-player
// relative-seat one-hots must be reached for every opponent seat.
func TestFeatFourSeatFDN(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	var total FeatStats
	pods := 0
	ps := Pairings(pool, 16, 13)
	for _, fs := range []policynet.FeatureSet{policynet.FeaturesEntity, policynet.FeaturesPod} {
		setPods := 0
		for i := 0; i+1 < len(ps) && setPods < 6; i += 2 {
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
			res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 4000, MaxTurns: 40, Check: FeatCheckFS(fp, &st, fs)})
			CloseSide(k)
			pods++
			setPods++
			total.Decisions += st.Decisions
			total.Options += st.Options
			for j, n := range st.EntHit {
				total.EntHit[j] += n
			}
			if d := res.Div; d != nil {
				if d.What == "features" || d.Field == "panic" {
					t.Errorf("pod %v (%s): after %d compared decisions: %s %s: %s", names, fs, st.Decisions, d.What, d.Field, d.Detail)
				} else {
					t.Logf("pod %v: rules divergence after %d compared decisions: %s %s", names, st.Decisions, d.What, d.Field)
				}
			}
		}
	}
	if pods == 0 || total.Decisions < 400 {
		t.Fatalf("%d pods, %d compared decisions", pods, total.Decisions)
	}
	for r := 0; r < 4; r++ {
		if total.EntHit[policynet.PodEntCtrl0+r] == 0 {
			t.Errorf("no pod entity card was controlled by relative seat %d", r)
		}
		if r > 0 && total.EntHit[policynet.PodEntAttacked0+r] == 0 {
			t.Errorf("no pod entity card attacked relative seat %d", r)
		}
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

// TestFeatFourSeatCommanderPods is TestFeatFourSeatVanilla over four-seat
// Commander pods, each under FeaturesEntity and FeaturesPod: the cmd5 decks cut to 40 cards (MiniCommanderDeck), as
// kfuzz -kernel cmd5 -seats 4 -cmd-mini 40 plays them, with FeatCheck on, so
// the Commander block (commander damage per relative seat, casts, the
// commander on the battlefield, the left-the-game bit) and the command zone
// are compared with policynet's encoding of gorge's view at every decision.
// A rules divergence ends a pod's comparison and is logged (the lockstep
// gate owns those); a feature mismatch or a panic fails, naming the field.
//
// Pods 4 and 7 are not among them: their Calling All Angels deck puts Tome
// of Legends on the battlefield, whose PAGE counters are an unfeaturized
// kernel kind (kop.CtrXNames) that gorge's counter string and mz counter
// rows list and the kernel encoders do not yet (the kernel keeps no "seen"
// record of those kinds, which gorge's list of a removed kind at 0 needs).
func TestFeatFourSeatCommanderPods(t *testing.T) {
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
	var total FeatStats
	pods := 0
	for pi, i := range []int{0, 1, 2, 3, 5, 6, 8, 10, 0, 1, 2, 3, 5, 6, 8, 10} {
		fs := policynet.FeaturesEntity
		if pi >= 8 {
			fs = policynet.FeaturesPod
		}
		p := PodAt(pool, i, 1, 4)
		m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		fp, err := FeatPool(c.Reg, tbl, nil)
		if err != nil {
			t.Fatal(err)
		}
		var st FeatStats
		rc := RunConfig{MaxDecisions: 3000, MaxTurns: 60, Check: FeatCheckFS(fp, &st, fs)}
		k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		pol := AffordableTargetPolicy(RandomPolicy(p.Seed))
		if i%2 == 1 {
			pol = HeuristicPolicy(p.Seed)
		}
		res := Lockstep(m, p.Seed, k, pol, rc)
		CloseSide(k)
		pods++
		total.Decisions += st.Decisions
		total.Options += st.Options
		if d := res.Div; d != nil {
			if d.What == "features" || d.Field == "panic" {
				t.Errorf("pod %d %v (%s): after %d compared decisions: %s %s: %s", i, p.Decks(), fs, st.Decisions, d.What, d.Field, d.Detail)
			} else {
				t.Logf("pod %d: rules divergence after %d compared decisions: %s %s", i, st.Decisions, d.What, d.Field)
			}
		}
	}
	if pods == 0 || total.Decisions < 2000 {
		t.Fatalf("%d pods, %d compared decisions", pods, total.Decisions)
	}
	t.Logf("%d pods: compared %d decisions, %d options", pods, total.Decisions, total.Options)
}
