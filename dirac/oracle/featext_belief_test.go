package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// beliefOf is a belief of 4 bombs and 4 answers (the scenario's counter; its table holds no kill) over sc's pool, and the
// bomb's id.
func beliefOf(t *testing.T, sc *FeatScenario) ([]uint8, kop.CardID) {
	t.Helper()
	const n = 1 << 14 // past any pool: an id past the table is a none card
	b := make([]uint8, n)
	var bomb kop.CardID
	nb, nk := 0, 0
	for id := kop.CardID(0); int(id) < n; id++ {
		ef := sc.Pool.Effect(id)
		switch {
		case nb == 0 && feat.StrengthBucket(sc.Pool.PrintedStrength(id)) == 3 && ef.Class != feat.EffCounter:
			bomb, nb = id, 1
			b[id] = 4
		case nk == 0 && ef.Class == feat.EffCounter: // the scenario's only answer is the counter-creature
			nk = 1
			b[id] = 4
		}
	}
	if nb == 0 || nk == 0 {
		t.Fatalf("no bomb (%d) or answer (%d) in the pool", nb, nk)
	}
	return b, bomb
}

func beliefScenarios(t *testing.T) map[string]*FeatScenario {
	c, _ := testCorpus(t)
	ss, err := FeatScenarios(rawSet("fdn"), c)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]*FeatScenario{}
	for i := range ss {
		by[ss[i].Name] = &ss[i]
	}
	return by
}

func TestExtBeliefCounts(t *testing.T) {
	by := beliefScenarios(t)
	pre, res := by["bomb-precast"], by["bomb-resolved"]
	belief, bomb := beliefOf(t, pre)
	enc := func(sc *FeatScenario, bel []uint8) []feat.Feature {
		e := feat.NewEncoder(sc.Pool)
		e.SetExt(feat.ExtBelief)
		e.SetOppBelief(bel)
		return append([]feat.Feature(nil), e.EncodeStateFor(sc.State, &sc.Dec, sc.Seat).Ext...)
	}
	// the bomb's strength class counts as a threat; the counter is an answer
	visible := func(sc *FeatScenario) int {
		var p dirac.Projection
		sc.State.Project(&p)
		n := 0
		for i := range p.Objs {
			o := &p.Objs[i]
			if o.Card == bomb && o.Ctrl != sc.Seat && (o.Zone == kop.ZoneBattlefield || o.Zone == kop.ZoneGraveyard || o.Zone == kop.ZoneExile) {
				n++
			}
		}
		return n
	}
	vp, vr := visible(pre), visible(res)
	if vr != vp+1 {
		t.Fatalf("precondition: visible bombs precast/resolved %d/%d, want one more", vp, vr)
	}
	a, b := enc(pre, belief), enc(res, belief)
	if _, ok := extSlot(a, 496+max(4-vp, 0)); !ok {
		t.Errorf("precast: threat slot %d unset: %v", 496+max(4-vp, 0), a)
	}
	if _, ok := extSlot(b, 496+max(4-vr, 0)); !ok {
		t.Errorf("resolved: threat slot %d unset: %v", 496+max(4-vr, 0), b)
	}
	if _, ok := extSlot(a, 501+4); !ok {
		t.Errorf("precast: answer bucket 4 unset: %v", a)
	}
	if got := enc(pre, nil); extAny(got, 496, 511) {
		t.Errorf("no belief: B slots set: %v", got)
	}
	// clearing after setting
	e := feat.NewEncoder(pre.Pool)
	e.SetExt(feat.ExtBelief)
	e.SetOppBelief(belief)
	e.SetOppBelief(nil)
	if extAny(e.EncodeStateFor(pre.State, &pre.Dec, pre.Seat).Ext, 496, 511) {
		t.Error("cleared belief still writes B")
	}
}

func TestExtBeliefNeverReadsHiddenZones(t *testing.T) {
	by := beliefScenarios(t)
	sc := by["bomb-precast"]
	belief, _ := beliefOf(t, sc)
	opp := 1 - sc.Seat
	var pool []kop.CardID
	buf := make([]kop.CardID, 512)
	n := sc.State.HiddenCards(opp, buf)
	pool = append(pool, buf[:n]...)
	deal := func(seed uint64) (*dirac.State, []kop.CardID) {
		st, _ := fork(sc.State, &sc.Dec)
		r := dirac.NewRand(seed, 1)
		st.DealHidden(opp, &r, slices.Clone(pool))
		out := make([]kop.CardID, 512)
		return st, slices.Clone(out[:st.HiddenCards(opp, out)])
	}
	s1, h1 := deal(1)
	s2, h2 := deal(2)
	if slices.Equal(h1, h2) {
		t.Fatal("precondition: the two deals are identical")
	}
	enc := func(st *dirac.State) []feat.Feature {
		e := feat.NewEncoder(sc.Pool)
		e.SetExt(feat.ExtBelief)
		e.SetOppBelief(belief)
		d := sc.Dec
		return append([]feat.Feature(nil), e.EncodeStateFor(st, &d, sc.Seat).Ext...)
	}
	a, b := enc(s1), enc(s2)
	if !extAny(a, 496, 511) {
		t.Fatal("precondition: no B slot set")
	}
	if !slices.Equal(a, b) {
		t.Errorf("B differs across hidden deals: %v vs %v", a, b)
	}
}
