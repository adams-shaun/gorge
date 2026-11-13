package oracle

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Group S (ExtStackTarget) on the constructed FDN positions (featscen.go).

func stackScenarios(t *testing.T) map[string]*FeatScenario {
	t.Helper()
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

func encStack(pool *feat.Pool, g feat.ExtGroups, s *dirac.State, d *dirac.Decision, seat uint8) []feat.Feature {
	e := feat.NewEncoder(pool)
	e.SetExt(g)
	return append([]feat.Feature(nil), e.EncodeStateFor(s, d, seat).Ext...)
}

// extIn lists the slots of x in lo..hi.
func extIn(x []feat.Feature, lo, hi int) []int {
	var out []int
	for _, f := range x {
		if int(f.Row) >= lo && int(f.Row) <= hi {
			out = append(out, int(f.Row))
		}
	}
	return out
}

// topEntry is the scenario's topmost stack entry, which must be the
// opponent's spell.
func topEntry(t *testing.T, sc *FeatScenario) (dirac.Projection, dirac.ProjStack) {
	t.Helper()
	var p dirac.Projection
	sc.State.Project(&p)
	if p.NStack == 0 {
		t.Fatalf("%s: empty stack", sc.Name)
	}
	top := p.Stack[p.NStack-1]
	if top.Kind != 0 || top.Ctrl == sc.Seat {
		t.Fatalf("%s: top entry kind %d ctrl %d, want the opponent's spell", sc.Name, top.Kind, top.Ctrl)
	}
	return p, top
}

var s2Q = [...]int{feat.EffKill: 0, feat.EffDamage: 1, feat.EffCounter: -1, feat.EffBounce: 2, feat.EffPump: 3, feat.EffDebuff: 4, feat.EffOther: -1}

func TestExtStackTargetClasses(t *testing.T) {
	by := stackScenarios(t)
	var s1 [][]feat.Feature
	for _, c := range []struct {
		name string
		tgt  int
	}{{"removal-at-my-creature", 2}, {"removal-at-my-land", 4}, {"damage-at-me", 0}} {
		sc := by[c.name]
		_, top := topEntry(t, sc)
		// the target is the encoded seat's
		tb := top.Targets[0]
		if dirac.TCode(tb) == dirac.PlayerTarget0+dirac.TCode(sc.Seat) {
		} else if dirac.TCode(tb) >= dirac.PlayerTarget0 {
			t.Fatalf("%s: target %#x is not the seat", c.name, tb)
		}
		if off := encStack(sc.Pool, 0, sc.State, &sc.Dec, sc.Seat); len(off) != 0 {
			t.Errorf("%s: off Ext has %d entries", c.name, len(off))
		}
		eff := int(sc.Pool.Effect(top.Card).Class)
		on := encStack(sc.Pool, feat.ExtStackTarget, sc.State, &sc.Dec, sc.Seat)
		want := 112 + (7+eff)*8 + c.tgt
		if got := extIn(on, 112, 223); !reflect.DeepEqual(got, []int{want}) {
			t.Errorf("%s: S1 slots %v, want [%d] (%s)", c.name, got, want, feat.ExtSlotName(uint16(want)))
		}
		s1 = append(s1, on)
	}
	for i := range s1 {
		for j := i + 1; j < len(s1); j++ {
			if reflect.DeepEqual(extIn(s1[i], 112, 223), extIn(s1[j], 112, 223)) {
				t.Errorf("scenarios %d and %d have equal S1", i, j)
			}
		}
	}
}

func TestExtKillCrossedWithStrength(t *testing.T) {
	by := stackScenarios(t)
	s2 := func(q, b, kills int) int { return 224 + ((((1*5+q)*2+0)*4+b)*2 + kills) }
	got := map[string][]feat.Feature{}
	for _, c := range []struct {
		name string
		b    int
	}{{"cheapkill-on-oneone", 0}, {"bigkill-on-bomb", 3}} {
		sc := by[c.name]
		p, top := topEntry(t, sc)
		ef := sc.Pool.Effect(top.Card)
		q := s2Q[ef.Class]
		if q < 0 {
			t.Fatalf("%s: spell class %d has no S2 q", c.name, ef.Class)
		}
		tgt := p.Objs[top.Targets[0]]
		if tgt.Zone != kop.ZoneBattlefield || tgt.Ctrl != sc.Seat {
			t.Fatalf("%s: target not my permanent", c.name)
		}
		kills := specKills(ef, int32(top.X), tgt, sc.State.Table().Cards[tgt.Card].Keywords&kop.KwIndestructible != 0)
		t.Logf("%s: roles %v, class %d amount %d, kills %d", c.name, sc.Roles, ef.Class, ef.Amount, kills)
		if ef.Class != feat.EffKill && ef.Class != feat.EffDamage && ef.Class != feat.EffDebuff {
			t.Fatalf("%s: the kill is class %d, not a removal class", c.name, ef.Class)
		}
		on := encStack(sc.Pool, feat.ExtStackTarget, sc.State, &sc.Dec, sc.Seat)
		got[c.name] = on
		want := s2(q, c.b, kills)
		if s := extIn(on, 224, 383); !reflect.DeepEqual(s, []int{want}) {
			t.Errorf("%s: S2 slots %v, want [%d] (%s)", c.name, s, want, feat.ExtSlotName(uint16(want)))
		}
	}
	a, b := extIn(got["cheapkill-on-oneone"], 224, 383), extIn(got["bigkill-on-bomb"], 224, 383)
	for _, x := range a {
		for _, y := range b {
			if x == y {
				t.Errorf("slot %d set in both", x)
			}
		}
	}
	if _, ok := extSlot(got["bigkill-on-bomb"], 384); !ok {
		t.Errorf("bigkill-on-bomb: 384 unset: %v", got["bigkill-on-bomb"])
	}
	if _, ok := extSlot(got["cheapkill-on-oneone"], 384); ok {
		t.Errorf("cheapkill-on-oneone: 384 set: %v", got["cheapkill-on-oneone"])
	}
}

// specKills is resolves-kills (feat's resolvesKills) restated over the projection (printed
// indestructible: the scenarios grant no keywords).
func specKills(ef feat.EffectSpec, x int32, t dirac.ProjObj, indestr bool) int {
	rem := int32(t.Tough) - int32(t.Damage)
	ok := false
	switch ef.Class {
	case feat.EffKill:
		ok = ef.Exile || !indestr
	case feat.EffDamage:
		amt := int32(ef.Amount)
		if ef.Amount == -1 {
			amt = x
		}
		ok = ef.Amount >= -1 && amt >= rem && !indestr
	case feat.EffDebuff:
		ok = -int32(ef.NumDef) >= rem
	}
	if ok {
		return 1
	}
	return 0
}

// castCounterOnTop casts the seat's hand counter (an EffCounter card) on
// the opponent's topmost spell and plays on to the next priority decision
// with the counter on the stack.
func castCounterOnTop(t *testing.T, sc *FeatScenario) (*dirac.State, *dirac.Decision) {
	t.Helper()
	st, d := fork(sc.State, &sc.Dec)
	var p dirac.Projection
	st.Project(&p)
	victim := uint8(p.Stack[p.NStack-1].Source)
	var cast *dirac.Action
	for _, a := range d.Options[:d.N] {
		o := &p.Objs[a.Source()]
		if a.Kind() == dirac.AkCast && a.Mode() == 0 && o.Zone == kop.ZoneHand && sc.Pool.Effect(o.Card).Class == feat.EffCounter {
			a := a
			cast = &a
			break
		}
	}
	if cast == nil {
		t.Fatalf("%s: no castable counter", sc.Name)
	}
	st.Apply(*cast)
	for n := 0; n < 50; n++ {
		st.StepInto(d)
		st.Project(&p)
		switch d.Kind {
		case dirac.DkTarget:
			pick := -1
			for i, a := range d.Options[:d.N] {
				if a.Target() == dirac.TCode(victim) {
					pick = i
				}
			}
			if pick < 0 {
				t.Fatalf("%s: the counter cannot target the spell", sc.Name)
			}
			st.Apply(d.Options[pick])
			continue
		case dirac.DkPriority:
			if top := p.Stack[p.NStack-1]; p.NStack >= 2 && top.Ctrl == sc.Seat && sc.Pool.Effect(top.Card).Class == feat.EffCounter {
				if top.Targets[0] != victim {
					t.Fatalf("%s: counter targets %d, want %d", sc.Name, top.Targets[0], victim)
				}
				return st, d
			}
		case dirac.DkNone:
			t.Fatalf("%s: game over", sc.Name)
		}
		st.Apply(d.Options[0])
	}
	t.Fatalf("%s: the counter never reached the stack", sc.Name)
	return nil, nil
}

func TestExtCounterStrength(t *testing.T) {
	by := stackScenarios(t)
	bomb := by["bomb-on-stack"]
	s, d := castCounterOnTop(t, bomb)
	x := encStack(bomb.Pool, feat.ExtStackTarget, s, d, bomb.Seat)
	if got := extIn(x, 392, 399); !reflect.DeepEqual(got, []int{392 + 3}) {
		t.Errorf("bomb-on-stack + counter: 392-399 %v, want [395]: %v", got, x)
	}
	ck := by["cheapkill-on-oneone"]
	_, top := topEntry(t, ck)
	mv := int32(s.Table().Cards[top.Card].Cost.MV())
	if mv > 2 {
		t.Fatalf("cheap kill mv %d > 2", mv)
	}
	s2, d2 := castCounterOnTop(t, ck)
	y := encStack(ck.Pool, feat.ExtStackTarget, s2, d2, ck.Seat)
	want := 392 + int(feat.StrengthBucket(2*mv))
	if got := extIn(y, 392, 399); !reflect.DeepEqual(got, []int{want}) {
		t.Errorf("cheapkill-on-oneone + counter: 392-399 %v, want [%d]: %v", got, want, y)
	}
	if reflect.DeepEqual(extIn(x, 392, 399), extIn(y, 392, 399)) {
		t.Error("the two counter positions are equal in 392-399")
	}
}

func TestExtStackTargetEqualAcrossPaths(t *testing.T) {
	by := stackScenarios(t)
	sAny := 0
	for _, n := range FeatScenarioNames {
		sc := by[n]
		mk := func() *feat.Encoder {
			e := feat.NewEncoder(sc.Pool)
			e.SetExt(feat.ExtAll)
			return e
		}
		x := append([]feat.Feature(nil), mk().EncodeStateFor(sc.State, &sc.Dec, sc.Seat).Ext...)
		y := append([]feat.Feature(nil), mk().EncodeStateIncFor(sc.State, &sc.Dec, sc.Seat).Ext...)
		z := mk().EncodeParts(sc.State, &sc.Dec, sc.Seat).Ext
		if !reflect.DeepEqual(x, y) || !reflect.DeepEqual(x, z) {
			t.Errorf("%s: Ext differs across paths:\nfull  %v\ninc   %v\nparts %v", n, x, y, z)
		}
		if extAny(x, 112, 399) {
			sAny++
		}
	}
	if sAny < 5 {
		t.Fatalf("only %d scenarios set an S slot", sAny)
	}
}
