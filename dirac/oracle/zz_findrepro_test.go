package oracle

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// throwaway: find games that cast a target card and print the window
func TestZZFindRepro(t *testing.T) {
	pool, target := os.Getenv("ZZ_POOL"), os.Getenv("ZZ_CARD")
	if pool == "" {
		t.Skip()
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
	c, err := OpenCorpus(corpusDirOrSkip(t), "")
	if err != nil {
		t.Fatal(err)
	}
	dealt, files, err := SynthPool(reg, set, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(files)
	ps := PairingsFrom(dealt, 0, 400, 1)
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, MaxDecisions: 20000}
	found := 0
	for _, p := range ps {
		if !strings.Contains(fmt.Sprint(p), "") {
			continue
		}
		m, tbl, err := SetMatchup(set, c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		var names []string
		has := false
		for s := 0; s < 2; s++ {
			for _, cd := range m.Decks[s] {
				names = append(names, cardName(cd))
				if cardName(cd) == target {
					has = true
				}
			}
		}
		if !has {
			continue
		}
		k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
		if err != nil {
			continue
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		CloseSide(k)
		rd := actionStrings(res.Actions)
		for i, a := range rd {
			if !strings.HasPrefix(a, "cast(#") {
				continue
			}
			var ix, mode int
			var rest string
			fmt.Sscanf(strings.TrimPrefix(a, "cast(#"), "%d,%s", &ix, &rest)
			_ = mode
			if ix >= len(names) || names[ix] != target {
				continue
			}
			hi := min(len(rd), i+10)
			t.Logf("pair %d %s vs %s div=%v at %d: %v", p.Index, p.A, p.B, res.Div != nil, i, rd[i:hi])
			found++
			if os.Getenv("ZZ_WRITE") == fmt.Sprintf("%d:%d", p.Index, i) {
				n := i + 10
				if v := os.Getenv("ZZ_CUT"); v != "" {
					fmt.Sscanf(v, "%d", &n)
				}
				res.Actions = res.Actions[:n]
				res.Div = nil
				rc2 := rc
				r := NewRepro(m, p.Seed, rc2, &res)
				r.Kernel = "synth " + pool
				if err := r.Write(os.Getenv("ZZ_OUT")); err != nil {
					t.Fatal(err)
				}
			}
		}
		if found > 12 {
			break
		}
	}
}
