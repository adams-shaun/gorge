package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAnyColourTapExtraAnswersTheKickedPips: a kicked cast announces the
// printed pips and the kicker payment's, so the any-colour tap extra (Dawn's
// Reflection) answers a kicked Bloodchief's Thirst ({2}{B}{B}) with {B}{W}
// beside the Swamp's {B}, not the {W}{W} its base {B} alone leaves owed. The
// search that prices the kicked offer floats those answers, so with the second
// {B} missing a Swamp, a Forest and the aura's two mana (four mana) never paid
// the cast (pool dr-mbc-abbb pair 77, decision 290).
func TestAnyColourTapExtraAnswersTheKickedPips(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{
		{Name: "Swamp", Count: 1}, {Name: "Forest", Count: 1}, {Name: "Bloodchief's Thirst", Count: 1},
		{Name: "Dawn's Reflection", Count: 1}, {Name: "Blossom Dryad", Count: 1},
	}
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in the mbc table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	// the Dryad (mana value 3) is a legal target for the kicked cast alone, so
	// its mode-1 offer is the only Thirst offer on these boards
	for _, v := range []struct {
		name          string
		auraCtl       int // 1: the aura is p0's, 2: p1's
		forests, pool int
		want          bool
	}{
		{"swamp+forest+pool under own aura", 1, 1, 1, true},
		{"swamp+forest+pool under foreign aura", 2, 1, 1, true},
		{"swamp+forest under own aura", 1, 1, 0, true},
		{"swamp+forest under foreign aura", 2, 1, 0, true},
		{"swamp+pool under own aura", 1, 0, 1, true},
		{"swamp alone under own aura: three mana", 1, 0, 0, false},
	} {
		sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
		swamp := sc.perm(id["Swamp"], 0)
		for i := 0; i < v.forests; i++ {
			sc.perm(id["Forest"], 0)
		}
		aura := sc.perm(id["Dawn's Reflection"], uint8(v.auraCtl-1))
		sc.s.attachTo(sc.k, sc.slot(aura), sc.slot(swamp))
		sc.perm(id["Blossom Dryad"], 1)
		thirst := sc.hand(id["Bloodchief's Thirst"], 0)
		sc.s.pl[0].pool[kop.W] = uint8(v.pool)
		if !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(swamp)) {
			t.Fatalf("%s: precondition: the aura'd Swamp is not a Taps-deferred gap", v.name)
		}
		d := sc.s.Step()
		got := false
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(thirst)) {
				if a.Mode()&3 != 1 {
					t.Fatalf("%s: a plain cast of the Dryad's removal is offered: %v", v.name, a)
				}
				got = true
			}
		}
		if got != v.want {
			t.Errorf("%s: kicked Bloodchief's Thirst offered = %v, want %v", v.name, got, v.want)
		}
	}
}

// TestAnyColourTapExtraFirstAnswer pins anyColFirst, the first answer of the
// any-colour extra's ask in gorge's prefix search: W in every unit but the
// last, which takes the lowest needed colour (W when it is needed or none is).
func TestAnyColourTapExtraFirstAnswer(t *testing.T) {
	need := func(cols ...int) (n [kop.NumManaKinds]bool) {
		for _, c := range cols {
			n[c] = true
		}
		return n
	}
	for _, v := range []struct {
		name  string
		need  [kop.NumManaKinds]bool
		units int
		want  [3]uint8
	}{
		{"nothing needed", need(), 2, [3]uint8{kop.W, kop.W}},
		{"black needed", need(kop.B), 2, [3]uint8{kop.W, kop.B}},
		{"red and green needed", need(kop.R, kop.G), 2, [3]uint8{kop.W, kop.R}},
		{"white needed", need(kop.W, kop.B), 2, [3]uint8{kop.W, kop.W}},
		{"colourless only", need(kop.C), 2, [3]uint8{kop.W, kop.W}},
		{"one unit", need(kop.G), 1, [3]uint8{kop.G}},
		{"three units", need(kop.U), 3, [3]uint8{kop.W, kop.W, kop.U}},
	} {
		if got := anyColFirst(&v.need, v.units); got != v.want {
			t.Errorf("%s: anyColFirst = %v, want %v", v.name, got, v.want)
		}
	}
}

// TestAnyColourTapExtraSecondAnswerOwesTheKicker: the second answer (the run
// deferGapSearch makes when the first prices nothing) is what the play still
// owes. A Swamp under Dawn's Reflection tapped for a kicked Bloodchief's
// Thirst floats {B} and the extra {B}{W} (owed {B}{B} less the Swamp's {B}),
// where the first answer leaves {B} and {W}{W}.
func TestAnyColourTapExtraSecondAnswerOwesTheKicker(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{
		{Name: "Swamp", Count: 1}, {Name: "Bloodchief's Thirst", Count: 1}, {Name: "Dawn's Reflection", Count: 1},
	}
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in the mbc table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, second := range []bool{false, true} {
		sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
		swamp := sc.perm(id["Swamp"], 0)
		aura := sc.perm(id["Dawn's Reflection"], 0)
		sc.s.attachTo(sc.k, sc.slot(aura), sc.slot(swamp))
		thirst := sc.hand(id["Bloodchief's Thirst"], 0)
		sc.s.setAnyColOwedRun(second)
		sc.s.tapDeferFloat(sc.k, 0, sc.slot(swamp), 0, MakeAction(AkCast, TCode(thirst), 0, 1))
		want := [kop.NumManaKinds]uint8{kop.W: 2, kop.B: 1}
		if second {
			want = [kop.NumManaKinds]uint8{kop.W: 1, kop.B: 2}
		}
		if got := sc.s.pl[0].pool; got != want {
			t.Errorf("second answer %v: pool %v, want %v", second, got, want)
		}
	}
}
