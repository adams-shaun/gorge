package oracle

import (
	"sort"
	"strings"
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

var relationsKeywords = []string{
	"Toxic", "Horsemanship", "Morph", "Megamorph", "Suspend", "Cascade", "Convoke", "Foretell", "Madness", "Multikicker",
	// T13b-2's records: the keywords whose printed face a lowered card can
	// carry and the setlists hold (Cycling, Ward, Enchant, Unearth). The
	// others have no carrier anywhere the pools draw from: Flanking, Disturb,
	// Mutate and Phasing because their K: line refuses the whole face, and
	// Decayed, Persist and Doctor's companion because no setlist names a card
	// printing them — so no focus seed can read those words true.
	"Cycling", "Ward", "Enchant", "Unearth",
}

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

// relationsSeedPools are the tail pools tried first: they held carriers of every keyword when the seed was cut.
// A tail pool is the names that hash to it, so a newly lowered card can move a carrier's whole bucket balance;
// 11-24 no listed pool dealt a Convoke carrier any more and main went red. relationsPools therefore falls back to
// every other tail pool in name order, and the test stops as soon as each atom is read both ways.
var relationsSeedPools = []string{"tail-34", "tail-10", "tail-28", "tail-21", "tail-01", "tail-53"}

// relationsPools is relationsSeedPools, then the corpus's other tail pools in name order.
func relationsPools(t *testing.T) []string {
	t.Helper()
	out := append([]string(nil), relationsSeedPools...)
	seen := map[string]bool{}
	for _, n := range out {
		seen[n] = true
	}
	var rest []string
	for _, n := range cardsettest.SynthNames(t) {
		if strings.HasPrefix(n, "tail-") && !seen[n] {
			rest = append(rest, n)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// relationsProven reports whether every atom has been read both ways.
func relationsProven(st *AtomStats, atoms []FilterAtom) bool {
	for _, a := range atoms {
		if st.True[a.Name] == 0 || st.False[a.Name] == 0 {
			return false
		}
	}
	return true
}

// TestAtomDiffRelationsFocus: the printed-keyword atoms agree between gorge's
// matcher and the kernel over focus decks of tail pools (it stops once every atom is read both ways), and each is read
// both ways by them (the proof the seed sets lack).
func TestAtomDiffRelationsFocus(t *testing.T) {
	atoms := relationsKeywordAtoms()
	total := NewAtomStats()
	games := 0
pools:
	for _, name := range relationsPools(t) {
		reg := synthReg(t)
		set := cardsettest.Synth(t, name)
		focus := relationsFocus(t, reg, set)
		if len(focus) == 0 {
			continue // a pool with no lowered carrier proves nothing; the totals below decide
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
			rc := RunConfig{Config: cfg, MaxDecisions: atomMaxDecisions, MaxTurns: 60, Check: AtomCheckFor(st, atoms, nil)}
			res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
			CloseSide(k)
			games++
			if d := res.Div; d != nil && d.What == "atom" {
				t.Errorf("%s %s vs %s: decision %d: %s: %s", name, p.A, p.B, d.At, d.Field, d.Detail)
			}
			total.Add(st)
			if relationsProven(total, atoms) {
				break pools // every atom is read both ways: more games only cost time (TF-05)
			}
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
