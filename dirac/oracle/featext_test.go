package oracle

import (
	"math"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

func extSlot(x []feat.Feature, slot int) (float32, bool) {
	for _, f := range x {
		if int(f.Row) == slot {
			return f.Value, true
		}
	}
	return 0, false
}

func extAny(x []feat.Feature, lo, hi int) bool {
	for _, f := range x {
		if int(f.Row) >= lo && int(f.Row) <= hi {
			return true
		}
	}
	return false
}

// extPositions calls f at every decision of n vanilla random games
// (stepped with the kernel's random policy), up to max decisions in all.
func extVanillaPositions(t *testing.T, pool *feat.Pool, games, max int, f func(s *dirac.State, d *dirac.Decision)) {
	t.Helper()
	n := 0
	for g := 0; g < games; g++ {
		a, b := vanilla.Decks[g%4], vanilla.Decks[(g/4)%4]
		s := dirac.NewConfig(vanilla.Table, a, b, uint64(g), dirac.Config{Mulligans: uint8(g % 3)})
		pol := dirac.NewRandomPolicy(uint64(g) * 7919)
		var d dirac.Decision
		for ; n < max; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			f(s, &d)
			s.Apply(pol.Pick(&d))
		}
	}
	if n == 0 {
		t.Fatal("no decisions visited")
	}
}

// extOffCheck encodes one position with a never-SetExt encoder and one that
// was ExtAll then 0, on all three paths.
func extOffCheck(t *testing.T, name string, plain, toggled *feat.Encoder, s *dirac.State, d *dirac.Decision, seat uint8) {
	t.Helper()
	a, b := plain.EncodeStateFor(s, d, seat), toggled.EncodeStateFor(s, d, seat)
	if a.Dense != b.Dense || !reflect.DeepEqual(a.Sparse, b.Sparse) || !reflect.DeepEqual(a.Cards, b.Cards) || len(a.Ext) != 0 || len(b.Ext) != 0 {
		t.Fatalf("%s: EncodeState differs or Ext non-empty (%d, %d)", name, len(a.Ext), len(b.Ext))
	}
	ai, bi := plain.EncodeStateIncFor(s, d, seat), toggled.EncodeStateIncFor(s, d, seat)
	if ai.Dense != bi.Dense || !reflect.DeepEqual(ai.Sparse, bi.Sparse) || !reflect.DeepEqual(ai.Cards, bi.Cards) || len(ai.Ext) != 0 || len(bi.Ext) != 0 {
		t.Fatalf("%s: EncodeStateInc differs or Ext non-empty", name)
	}
	ap, bp := plain.EncodeParts(s, d, seat), toggled.EncodeParts(s, d, seat)
	if ap.Dense != bp.Dense || !reflect.DeepEqual(ap.Objs, bp.Objs) || !reflect.DeepEqual(ap.Fresh, bp.Fresh) ||
		!reflect.DeepEqual(ap.Ents, bp.Ents) || len(ap.Ext) != 0 || len(bp.Ext) != 0 {
		t.Fatalf("%s: EncodeParts differs or Ext non-empty", name)
	}
}

func TestExtOffBitIdentical(t *testing.T) {
	c, _ := testCorpus(t)
	ss, err := FeatScenarios(rawSet("fdn"), c)
	if err != nil {
		t.Fatal(err)
	}
	for i := range ss {
		sc := &ss[i]
		plain, toggled := feat.NewEncoder(sc.Pool), feat.NewEncoder(sc.Pool)
		toggled.SetExt(feat.ExtAll)
		toggled.SetExt(0)
		extOffCheck(t, sc.Name, plain, toggled, sc.State, &sc.Dec, sc.Seat)
		// the control: the block is not vacuously empty when on
		on := feat.NewEncoder(sc.Pool)
		on.SetExt(feat.ExtAll)
		if len(on.EncodeStateFor(sc.State, &sc.Dec, sc.Seat).Ext) == 0 {
			t.Errorf("%s: ExtAll writes nothing", sc.Name)
		}
	}
	pool, err := FeatPool(c.Reg, vanilla.Table, nil)
	if err != nil {
		t.Fatal(err)
	}
	plain, toggled := feat.NewEncoder(pool), feat.NewEncoder(pool)
	toggled.SetExt(feat.ExtAll)
	toggled.SetExt(0)
	seen := 0
	extVanillaPositions(t, pool, 8, 200, func(s *dirac.State, d *dirac.Decision) {
		seen++
		extOffCheck(t, "vanilla", plain, toggled, s, d, d.Player)
	})
	if seen != 200 {
		t.Fatalf("visited %d decisions, want 200", seen)
	}
}

func TestExtEqualAcrossPaths(t *testing.T) {
	c, _ := testCorpus(t)
	pool, err := FeatPool(c.Reg, vanilla.Table, nil)
	if err != nil {
		t.Fatal(err)
	}
	mk := func() *feat.Encoder {
		e := feat.NewEncoder(pool)
		e.SetExt(feat.ExtEconomy | feat.ExtTurn)
		return e
	}
	full, inc, parts := mk(), mk(), mk()
	nonEmpty, n := 0, 0
	for g := 0; g < 4; g++ {
		a, b := vanilla.Decks[g%4], vanilla.Decks[(g/4+1)%4]
		s := dirac.NewConfig(vanilla.Table, a, b, uint64(g), dirac.Config{})
		pol := dirac.NewRandomPolicy(uint64(g) * 7919)
		var d dirac.Decision
		for k := 0; k < 20000; k++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			x := append([]feat.Feature(nil), full.EncodeState(s, &d).Ext...)
			y := append([]feat.Feature(nil), inc.EncodeStateInc(s, &d).Ext...)
			z := parts.EncodeParts(s, &d, d.Player).Ext
			n++
			if len(x) > 0 {
				nonEmpty++
			}
			if !reflect.DeepEqual(x, y) || !reflect.DeepEqual(x, z) {
				t.Fatalf("game %d decision %d: Ext differs across paths:\nfull  %v\ninc   %v\nparts %v", g, k, x, y, z)
			}
			s.Apply(pol.Pick(&d))
		}
	}
	if n == 0 || nonEmpty != n {
		t.Fatalf("%d of %d decisions have a non-empty Ext", nonEmpty, n)
	}
}

func TestExtCounteredBombEconomy(t *testing.T) {
	c, _ := testCorpus(t)
	ss, err := FeatScenarios(rawSet("fdn"), c)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]*FeatScenario{}
	for i := range ss {
		by[ss[i].Name] = &ss[i]
	}
	enc := func(name string, g feat.ExtGroups, seat int) []feat.Feature {
		sc := by[name]
		e := feat.NewEncoder(sc.Pool)
		e.SetExt(g)
		if seat < 0 {
			seat = int(sc.Seat)
		}
		return append([]feat.Feature(nil), e.EncodeStateFor(sc.State, &sc.Dec, uint8(seat)).Ext...)
	}
	names := []string{"bomb-precast", "bomb-on-stack", "bomb-countered", "bomb-resolved"}
	for _, n := range names {
		if got := enc(n, 0, -1); len(got) != 0 {
			t.Errorf("%s: Ext off has %d entries", n, len(got))
		}
	}
	const on = feat.ExtEconomy | feat.ExtTurn
	pre, stk, ctr, res := enc(names[0], on, -1), enc(names[1], on, -1), enc(names[2], on, -1), enc(names[3], on, -1)
	// my open mana is the untapped lands (Dense[37]) and my hand size Dense[29]; the
	// the 3 open lands after the counter is paid (measured: 6 before, 3 after)
	dense := func(name string) (open, hand int) {
		sc := by[name]
		st := feat.NewEncoder(sc.Pool).EncodeStateFor(sc.State, &sc.Dec, sc.Seat)
		return int(st.Dense[37]), int(st.Dense[29])
	}
	openPre, handPre := dense(names[0])
	openStk, _ := dense(names[1])
	openCtr, _ := dense(names[2])
	if openPre <= openCtr || openPre != openStk || openCtr < 1 {
		t.Fatalf("open lands precast/stack/countered %d/%d/%d: the countering spell was not paid", openPre, openStk, openCtr)
	}

	// precast: three open lands on the opponent's turn, a castable counter, an empty stack
	if by[names[0]].State.Turn() < 1 {
		t.Fatal("bad scenario turn")
	}
	if _, ok := extSlot(pre, 46+openPre); !ok {
		t.Errorf("precast: open mana %d unset: %v", openPre, pre)
	}
	// every card of the counter player's hand is a castable counter
	if v, ok := extSlot(pre, 84); !ok || v < 1 || int(v) > handPre {
		t.Errorf("precast: castable counter slot 84 = %v,%v want 1..%d", v, ok, handPre)
	}
	if extAny(pre, 64, 83) {
		t.Errorf("precast: stack slots set: %v", pre)
	}
	// on the stack: the bomb is the opponent's entry
	if extAny(stk, 64, 72) {
		t.Errorf("on-stack: own stack-top slots set: %v", stk)
	}
	v83, ok := extSlot(stk, 83)
	if !ok || v83 <= 0 {
		t.Fatalf("on-stack: slot 83 unset: %v", stk)
	}
	mv := int(math.Round(float64(v83) * 10))
	if _, ok := extSlot(stk, 73+min(mv, 8)); !ok {
		t.Errorf("on-stack: slot %d (stack MV %d) unset: %v", 73+min(mv, 8), mv, stk)
	}
	if _, ok := extSlot(stk, 46+openStk); !ok {
		t.Errorf("on-stack: open mana %d unset: %v", openStk, stk)
	}
	// countered: nothing on the stack, my lands tapped
	if extAny(ctr, 64, 83) {
		t.Errorf("countered: stack slots set: %v", ctr)
	}
	if _, ok := extSlot(ctr, 46+openCtr); !ok {
		t.Errorf("countered: open mana %d unset: %v", openCtr, ctr)
	}
	b0, _ := extSlot(pre, 1)
	if b1, _ := extSlot(ctr, 1); b1 != b0 {
		t.Errorf("countered: opp bfmv %v, precast %v", b1, b0)
	}
	// resolved: the bomb is on their board
	b2, _ := extSlot(res, 1)
	if want := b0 + float32(mv)/10; math.Abs(float64(b2-want)) > 1e-6 || mv < 1 {
		t.Errorf("resolved: opp bfmv %v, want %v (bomb mv %d)", b2, want, mv)
	}
	// the same stack state, for the bomb's controller
	own := enc(names[1], on, int(1-by[names[1]].Seat))
	if _, ok := extSlot(own, 64+min(mv, 8)); !ok {
		t.Errorf("controller: slot %d unset: %v", 64+min(mv, 8), own)
	}
	if extAny(own, 73, 81) {
		t.Errorf("controller: opp stack-top slots set: %v", own)
	}
}
