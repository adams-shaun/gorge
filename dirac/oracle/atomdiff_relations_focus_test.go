package oracle

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The printed-keyword words of kop.FwWithKeyword (T13b) are read by no card of
// the three seed sets TestAtomCoverage sums (no deck there prints Toxic,
// Horsemanship, Morph ... both ways), so registering them would add twenty
// unproven pins. They are proven here instead, by a focus seed: synthetic
// tail pools whose decks are dealt around lowered cards that print the
// keywords, played under the same atom check over the keyword atoms alone.

var relationsKeywords = []string{"Toxic", "Horsemanship", "Morph", "Megamorph", "Suspend", "Cascade", "Convoke", "Foretell", "Madness", "Multikicker"}

// relationsKeywordAtoms is the with/without pair of each printed keyword.
func relationsKeywordAtoms() []FilterAtom {
	var out []FilterAtom
	for _, k := range relationsKeywords {
		for _, w := range []string{"Card.with" + k, "Card.without" + k} {
			out = append(out, FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomAnyZone})
		}
	}
	return out
}

// relationsFocus is the focus list over set: up to two lowered cards printing
// each keyword, in set order.
func relationsFocus(t *testing.T, reg *cards.Registry, set cardset.Set) []FocusCard {
	t.Helper()
	got := map[string]int{}
	var focus []FocusCard
	for i, n := range set.Names()[:set.NumCards()] {
		if set.Status(i) == cardset.StatusNotLowered || cardset.IsSynthBasic(n) {
			continue
		}
		c, ok := reg.Lookup(n)
		if !ok || len(c.Faces) == 0 {
			continue
		}
		for _, k := range relationsKeywords {
			if got[k] < 2 && c.Faces[0].HasKeyword(k) {
				got[k]++
				focus = append(focus, FocusCard{Name: n})
				break
			}
		}
	}
	return focus
}

// TestAtomDiffRelationsFocus: the printed-keyword atoms agree between gorge's
// matcher and the kernel over focus decks of five tail pools, and each is read
// both ways by them (the proof the seed sets lack).
func TestAtomDiffRelationsFocus(t *testing.T) {
	atoms := relationsKeywordAtoms()
	total := NewAtomStats()
	games := 0
	for _, name := range []string{"tail-34", "tail-10", "tail-28", "tail-21", "tail-01", "tail-53"} {
		reg := synthReg(t)
		set := cardsettest.Synth(t, name)
		focus := relationsFocus(t, reg, set)
		if len(focus) == 0 {
			t.Fatalf("%s: no lowered card prints a keyword of the family", name)
		}
		pool, files, err := FocusPool(reg, set, "f", focus, 6, 1, 8)
		if err != nil {
			t.Fatal(err)
		}
		c := &Corpus{Reg: reg, decks: map[string][]*cards.Card{}}
		c.AddDecks(files)
		cfg := Config{GorgeAutopaySurface: true}
		for i := 0; i < atomPairs; i++ {
			p := PairingAt(pool, i, 1)
			m, tbl, err := SetMatchup(set, c, p.A, p.B, 0)
			if err != nil {
				continue
			}
			k, err := SetSide(set, tbl, cfg)(m, p.Seed)
			if err != nil {
				t.Fatal(err)
			}
			st := NewAtomStats()
			st.Seed = p.Seed
			rc := RunConfig{Config: cfg, MaxDecisions: 2 * atomMaxDecisions, MaxTurns: 60, Check: AtomCheckFor(st, atoms, nil)}
			res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
			CloseSide(k)
			games++
			if d := res.Div; d != nil && d.What == "atom" {
				t.Errorf("%s %s vs %s: decision %d: %s: %s", name, p.A, p.B, d.At, d.Field, d.Detail)
			}
			total.Add(st)
		}
	}
	if games == 0 || total.Decisions == 0 {
		t.Fatalf("%d games, %d compared decisions", games, total.Decisions)
	}
	var unproven []string
	for _, a := range atoms {
		if total.True[a.Name] == 0 || total.False[a.Name] == 0 {
			unproven = append(unproven, a.Name)
		}
	}
	sort.Strings(unproven)
	for _, n := range unproven {
		t.Errorf("atom %s is not read both ways by the focus seed (true %d, false %d)", n, total.True[n], total.False[n])
	}
}
