package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Casualty (CR 702.153a, kw_casualty.go). No FDN card carries it, so the
// scenario gives BLB's Wear Down a Casualty threshold on a table built in
// the test (the cast, the election and the copy trigger do not read the
// card's text).

func casualtyScen(t *testing.T, n uint8, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	var deck []cardset.Entry
	set := cardsettest.Set(t, "blb")
	for _, nm := range names {
		if i, ok := set.Lookup(nm); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the BLB table", nm)
		}
		deck = append(deck, cardset.Entry{Name: nm, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tbl.Cards[id["Wear Down"]].Casualty = n
	return newScenTable(t, tbl, Config{NoElide: true}), id
}

// castOptions lists the cast options (plain or casualty) offered for o.
func (sc *scen) castOptions(o Obj) (acts []Action) {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; (a.Kind() == AkCast || a.Kind() == AkCastCasualty) && a.Source() == TCode(uint8(o)) {
			acts = append(acts, a)
		}
	}
	return
}

func (sc *scen) castOption(t *testing.T, o Obj, kind ActionKind, mode uint8) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == kind && a.Source() == TCode(uint8(o)) && a.Mode() == mode {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("cast of %d kind %d mode %d not offered: %v", o, kind, mode, d.Options[:d.N])
}

func TestCasualtySacrificePaidCopyResolvesFirst(t *testing.T) {
	sc, id := casualtyScen(t, 2, "Wear Down", "Forest", "Fountainport Bell", "Jolly Gerbils")
	for i := 0; i < 2; i++ {
		sc.perm(id["Forest"], 0)
	}
	gerbils := sc.perm(id["Jolly Gerbils"], 0)
	bell := sc.perm(id["Fountainport Bell"], 1)
	wd := sc.hand(id["Wear Down"], 0)
	casts := sc.castOptions(wd)
	if len(casts) != 2 || casts[0].Kind() != AkCast || casts[0].Mode() != 0 || casts[1].Kind() != AkCastCasualty {
		t.Fatalf("cast options %v, want the plain cast then the casualty cast", casts)
	}
	sc.castOption(t, wd, AkCastCasualty, 0)
	d := sc.s.Step()
	if d.Kind == DkChoose && d.Sub == ChGenGift {
		sc.s.Apply(MakeAction(AkChoose, 0, 0, 0)) // decline the gift
		d = sc.s.Step()
	}
	if d.Kind != DkChoose || d.Sub != ChGenCasualty || d.N != 1 || d.Options[0].Source() != TCode(uint8(gerbils)) {
		t.Fatalf("decision kind %d sub %d n %d, want the casualty election over the Gerbils", d.Kind, d.Sub, d.N)
	}
	if !sc.onBF(gerbils) {
		t.Fatal("the creature is sacrificed at the election; gorge keeps it until payment")
	}
	sc.s.Apply(d.Options[0])
	sc.do(t, AkTarget, scAny, TCode(bell))
	for i := 0; i < 20 && sc.s.sp < 2; i++ {
		d = sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.onBF(gerbils) || sc.s.objs[gerbils].zone != kop.ZoneGraveyard {
		t.Fatal("the casualty creature was not sacrificed at payment")
	}
	if sc.s.sp != 2 || sc.s.stack[1].kind != seTrigger || sc.s.stack[1].obj != wd {
		t.Fatalf("stack depth %d, want the spell with the casualty trigger above it", sc.s.sp)
	}
	// the trigger resolves: the copy elects inside it, then sits above the spell
	// and resolves first
	for i := 0; i < 10 && (sc.s.sp == 3 || sc.s.stack[sc.s.sp-1].kind == seTrigger); i++ {
		switch d := sc.s.Step(); d.Kind {
		case DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		case DkTarget:
			sc.s.Apply(d.Options[0])
		}
	}
	sc.s.Step()
	if sc.s.sp != 2 || sc.s.stack[0].obj != wd || !sc.s.isSpellCopy(sc.s.stack[1].obj) {
		t.Fatalf("stack depth %d, want the original under its copy: %+v", sc.s.sp, sc.s.stack[:sc.s.sp])
	}
}

func TestCasualtyNoQualifyingCreature(t *testing.T) {
	sc, id := casualtyScen(t, 3, "Wear Down", "Forest", "Fountainport Bell", "Jolly Gerbils")
	for i := 0; i < 2; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Jolly Gerbils"], 0) // power 2 < 3
	sc.perm(id["Fountainport Bell"], 1)
	wd := sc.hand(id["Wear Down"], 0)
	if casts := sc.castOptions(wd); len(casts) != 1 || casts[0].Kind() != AkCast || casts[0].Mode() != 0 {
		t.Fatalf("cast options %v, want only the plain cast", casts)
	}
}

func TestCasualtyPlainCastUnchanged(t *testing.T) {
	sc, id := casualtyScen(t, 2, "Wear Down", "Forest", "Fountainport Bell", "Jolly Gerbils")
	for i := 0; i < 2; i++ {
		sc.perm(id["Forest"], 0)
	}
	gerbils := sc.perm(id["Jolly Gerbils"], 0)
	sc.perm(id["Fountainport Bell"], 1)
	wd := sc.hand(id["Wear Down"], 0)
	sc.castOption(t, wd, AkCast, 0)
	for i := 0; i < 20; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChGenCasualty {
			t.Fatal("the plain cast asked the casualty election")
		}
		if d.Kind == DkPriority {
			break
		}
		if d.Kind == DkChoose && d.Sub == ChGenGift {
			sc.s.Apply(MakeAction(AkChoose, 0, 0, 0))
			continue
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.sp != 1 || !sc.onBF(gerbils) {
		t.Fatalf("stack depth %d, creature on battlefield %v; the plain cast is the spell alone", sc.s.sp, sc.onBF(gerbils))
	}
}
