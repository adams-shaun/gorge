package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestV2BlockFiresAndMatchesGorge is the load-bearing check for the v2 entity
// block (mana colour, pip counts and coarse category at slots 84..99).
//
// Two things must hold and neither is implied by the general oracle, which
// stops at the first divergence and would pass just as happily if the whole
// block were zero on both sides:
//
//  1. the block fires on real cards (every group has non-zero lanes), so a
//     recording that silently stopped writing it cannot ship as training data;
//  2. gorge's policynet encoder and dirac/feat write the SAME value in every
//     v2 slot, counted over the whole run rather than until the first error.
//
// A pack of zero columns trains fine and reports a flattering loss, so this
// is the test that would otherwise be missing.
func TestV2BlockFiresAndMatchesGorge(t *testing.T) {
	t.Parallel()

	c, _ := testCorpus(t)
	pool, err := FeatPool(c.Reg, vanilla.Table, nil)
	if err != nil {
		t.Fatal(err)
	}
	enc := feat.NewEncoder(pool)
	var cards, colour, pips, cat, mism int
	check := func(g *GorgeSide, k Side) *Divergence {
		r, ok := k.(*realSide)
		if !ok || r.d.N == 0 || g.fr == nil {
			return nil
		}
		gst, _ := GorgeFeatures(g.E, g.fr.d, nil)
		st := enc.EncodeState(&r.s, &r.d)
		if len(gst.Cards) != len(st.Cards) {
			return nil
		}
		for i := range st.Cards {
			gr, kr := gst.Cards[i].Raw, st.Cards[i].Raw
			for j := 84; j < 100; j++ {
				if j >= len(gr) || j >= len(kr) {
					continue
				}
				if gr[j] != kr[j] {
					mism++
				}
			}
			cards++
			for j := 84; j < 90; j++ {
				if kr[j] != 0 {
					colour++
				}
			}
			for j := 90; j < 96; j++ {
				if kr[j] != 0 {
					pips++
				}
			}
			for j := 96; j < 100; j++ {
				if kr[j] != 0 {
					cat++
				}
			}
		}
		return nil
	}
	cfg := RunConfig{}
	cfg.Check = check
	for g := 0; g < 3; g++ {
		m, err := KernelMatchup(c.Reg, vanilla.Table,
			[2]string{fmt.Sprint("vanilla", 0), fmt.Sprint("vanilla", 1)},
			[2][]kop.CardID{vanilla.Decks[0], vanilla.Decks[1]}, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := RealKernel(vanilla.Table, nil, cfg.Config)(m, uint64(9100+g))
		if err != nil {
			t.Fatal(err)
		}
		Lockstep(m, uint64(9100+g), k, RandomPolicy(uint64(9100+g)), cfg)
	}
	t.Logf("cards=%d colour=%d pips=%d category=%d mismatches=%d", cards, colour, pips, cat, mism)
	if cards == 0 || colour == 0 || pips == 0 || cat == 0 {
		t.Fatalf("v2 block did not fire: cards=%d colour=%d pips=%d cat=%d", cards, colour, pips, cat)
	}
	if mism != 0 {
		t.Fatalf("%d v2 slots disagree with gorge's encoder", mism)
	}
}

// TestV2BlockCategoriesFDN runs the same comparison over FDN pairings, whose
// decks carry removal, mana rocks and tricks the vanilla decks lack, and
// requires every category lane (removal, creature, ramp, trick) to fire.
func TestV2BlockCategoriesFDN(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	var lanes [4]int
	cards, mism, games := 0, 0, 0
	for _, p := range Pairings(pool, 10, 11) {
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		fp, err := FeatPool(c.Reg, tbl, nil)
		if err != nil {
			t.Fatal(err)
		}
		enc := feat.NewEncoder(fp)
		k, err := RealKernel(tbl, nil, RunConfig{}.Config)(m, p.Seed)
		if err != nil {
			continue
		}
		rc := RunConfig{}
		rc.Check = func(g *GorgeSide, ks Side) *Divergence {
			r, ok := ks.(*realSide)
			if !ok || r.d.N == 0 || g.fr == nil {
				return nil
			}
			gst, _ := GorgeFeatures(g.E, g.fr.d, nil)
			st := enc.EncodeState(&r.s, &r.d)
			if len(gst.Cards) != len(st.Cards) {
				return nil
			}
			for i := range st.Cards {
				gr, kr := gst.Cards[i].Raw, st.Cards[i].Raw
				for j := feat.EntityRawWidthRelEnd; j < feat.EntityRawWidth && j < len(gr); j++ {
					if gr[j] != kr[j] {
						mism++
					}
				}
				cards++
				for l := 0; l < 4; l++ {
					if kr[96+l] != 0 {
						lanes[l]++
					}
				}
			}
			return nil
		}
		Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		games++
	}
	t.Logf("games=%d cards=%d removal=%d creature=%d ramp=%d trick=%d mismatches=%d",
		games, cards, lanes[0], lanes[1], lanes[2], lanes[3], mism)
	if games == 0 {
		t.Skip("no FDN pairing built")
	}
	for l, n := range lanes {
		if n == 0 {
			t.Errorf("category lane %d never fired", 96+l)
		}
	}
	if mism != 0 {
		t.Fatalf("%d v2 slots disagree with gorge's encoder", mism)
	}
}
