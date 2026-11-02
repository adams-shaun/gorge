package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"

	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestFeatVanilla runs the real kernel in lockstep with gorge on the
// vanilla decks with FeatCheck on: at every decision before the game's
// first divergence the kernel's encoded network inputs must equal gorge's.
// It logs the comparison counts and the first feature mismatch of each
// game; it fails only on a feature mismatch.
func TestFeatVanilla(t *testing.T) {
	t.Parallel()

	c, _ := testCorpus(t)
	pool, err := FeatPool(c.Reg, vanilla.Table, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := RunConfig{}
	mk := RealKernel(vanilla.Table, nil, cfg.Config)
	var total FeatStats
	for g := 0; g < 24; g++ {
		a := g % len(vanilla.Decks)
		b := (a + 1 + g/len(vanilla.Decks)%(len(vanilla.Decks)-1)) % len(vanilla.Decks)
		seed := uint64(7000 + g)
		m, err := KernelMatchup(c.Reg, vanilla.Table, [2]string{fmt.Sprint("vanilla", a), fmt.Sprint("vanilla", b)},
			[2][]kop.CardID{vanilla.Decks[a], vanilla.Decks[b]}, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := mk(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var st FeatStats
		rc := cfg
		rc.Check = FeatCheck(pool, &st)
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		total.Decisions += st.Decisions
		total.Options += st.Options
		if d := res.Div; d != nil {
			t.Errorf("game %d (vanilla %d vs %d): after %d compared decisions: %s %s: %s", g, a, b, st.Decisions, d.What, d.Field, d.Detail)
		}
	}
	t.Logf("compared %d decisions, %d options", total.Decisions, total.Options)
}

// TestFeatFDN is TestFeatVanilla over random FDN pairings with the
// card table (the fdn pool): unsupported matchups are
// skipped.
func TestFeatFDN(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	cfg := RunConfig{}
	var total FeatStats
	games, skipped := 0, 0
	for _, p := range Pairings(pool, 40, 11) {
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			skipped++
			continue
		}
		fp, err := FeatPool(c.Reg, tbl, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(rawSet("fdn"), tbl, cfg.Config)(m, p.Seed)
		if err != nil {
			skipped++
			continue
		}
		var st FeatStats
		rc := cfg
		rc.Check = FeatCheck(fp, &st)
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		CloseSide(k)
		games++
		total.Decisions += st.Decisions
		total.Options += st.Options
		for i, ok := range st.Rel {
			total.Rel[i] = total.Rel[i] || ok
		}
		if d := res.Div; d != nil {
			t.Errorf("%s vs %s: after %d compared decisions: %s %s: %s", p.A, p.B, st.Decisions, d.What, d.Field, d.Detail)
		}
	}
	for i := 0; i < 15; i++ {
		if i == 8 { // entStackX; TestFeatEntityX casts an X spell for X >= 5
			continue
		}
		if !total.Rel[i] {
			t.Errorf("entity relational slot %d was never exercised", feat.EntityRawWidthV1+i)
		}
	}
	t.Logf("%d games (%d skipped): compared %d decisions, %d options; relational slots %v", games, skipped, total.Decisions, total.Options, total.Rel)
}

// xPolicy casts whenever it can and takes the largest X the cast offers, so
// a game reaches a stack card with X >= 5 (entity slot entStackX).
func xPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DChoose && len(c.D.Options) >= 6 {
			all := true
			for i, a := range c.D.Options {
				if a.Kind() != ActMode || int(a.Mode()) != i {
					all = false
					break
				}
			}
			if all {
				return c.D.Options[len(c.D.Options)-1]
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestFeatEntityX exercises the stack-X relational slot against gorge's
// reference encoder: an X spell cast for X >= 5 while it is on the stack.
func TestFeatEntityX(t *testing.T) {
	t.Parallel()

	c, _ := testFDNPool(t)
	m, tbl, err := SetMatchup(rawSet("fdn"), c, "FDN_top_00016_BG", "FDN_top_00016_BG", 0)
	if err != nil {
		t.Fatal(err)
	}
	fp, err := FeatPool(c.Reg, tbl, nil)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 64; seed++ {
		k, err := SetSide(rawSet("fdn"), tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var st FeatStats
		res := Lockstep(m, seed, k, xPolicy(seed), RunConfig{MaxDecisions: 20000, MaxTurns: 100, Check: FeatCheck(fp, &st)})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: %s %s: %s", seed, d.What, d.Field, d.Detail)
		}
		if st.Rel[8] { // entStackX = EntityRawWidthV1+8
			t.Logf("seed %d: stack X exercised", seed)
			return
		}
	}
	t.Fatal("entity slot entStackX was never nonzero")
}

// featBLBGame plays one BLB game with the network inputs compared at every
// decision, under the configuration of the pool runs.
func featBLBGame(t *testing.T, a, b string, seed uint64) {
	t.Helper()
	c, _ := testPoolCorpus(t, "blb")
	cfg := Config{GorgeAutopaySurface: true}
	m, tbl, err := SetMatchup(rawSet("blb"), c, a, b, 1)
	if err != nil {
		t.Fatal(err)
	}
	fp, err := FeatPool(c.Reg, tbl, nil)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(rawSet("blb"), tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	var st FeatStats
	res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100, Check: FeatCheck(fp, &st)})
	if d := res.Div; d != nil {
		t.Errorf("%s vs %s: after %d compared decisions: %s %s: %s", a, b, st.Decisions, d.What, d.Field, d.Detail)
	}
}

// TestFeatBLBCountedCDAOffBattlefield: Burrowguard Mentor in hand shows the
// number of creatures its owner controls, as gorge's view does.
func TestFeatBLBCountedCDAOffBattlefield(t *testing.T) {
	t.Parallel()
	featBLBGame(t, "BLB_top_33923_WG", "BLB_top_05385_WpB", 14986055305374995707)
}

// TestFeatBLBTapOtherManaOption: gorge's option list has no mana tap for a
// Baylen, the Haymaker without two untapped tokens.
func TestFeatBLBTapOtherManaOption(t *testing.T) {
	t.Parallel()
	featBLBGame(t, "BLB_top_00001_WRpBG", "BLB_top_05590_WBpR", 5078431631105097588)
}

// TestFeatXAtTheTargetAsk: gorge writes a spell's X as the cast commits, so a target
// ask after the X ask sees 0 in the stack card's X slot. Dealt decks, where X spells
// are common enough to reach it.
func TestFeatXAtTheTargetAsk(t *testing.T) {
	t.Parallel()
	c := &Corpus{Reg: synthReg(t), decks: map[string][]*cards.Card{}}
	pool, err := addSynthFDNDecks(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range Pairings(pool, 40, 11) {
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			t.Fatal(err)
		}
		fp, err := FeatPool(c.Reg, tbl, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(rawSet("fdn"), tbl, Config{})(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		var st FeatStats
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Check: FeatCheck(fp, &st)})
		CloseSide(k)
		if d := res.Div; d != nil && d.What == "features" {
			t.Errorf("%s vs %s: %s", p.A, p.B, d.Detail)
		}
	}
}
