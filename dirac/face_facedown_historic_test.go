package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A face-down spell is not historic (CR 708.4: it has no name, types or
// supertypes beyond 2/2 colourless creature, so not artifact, legendary or
// Saga): Glóin, Dwarf Emissary's "Whenever you cast a historic spell" must
// not queue for a face-down Morph cast of legendary Akroma, Angel of Fury.

// historicScen puts Glóin on the battlefield and a face-down cast of Akroma
// on the stack, paid from a floating {3}.
func historicScen(t *testing.T) (*scen, CardID, Obj, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Glóin, Dwarf Emissary", Count: 1},
		{Name: "Akroma, Angel of Fury", Count: 1},
		{Name: "Zoetic Cavern", Count: 18},
	}
	set := cardsettest.Set(t, "facedown")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Zoetic Cavern"], Config{NoElide: true})
	gloin := sc.perm(id["Glóin, Dwarf Emissary"], 0)
	ak := sc.hand(id["Akroma, Angel of Fury"], 0)
	// the face-down cast is priced from the floating pool alone
	sc.s.pl[0].pool[kop.G] = 3
	return sc, id["Akroma, Angel of Fury"], gloin, ak
}

// historicTrigFilter finds Glóin's SpellCast trigger and its filter head.
// The card must be lowered as the watcher this test needs, or the test says
// nothing about the kernel's queueing.
func historicTrigFilter(t *testing.T, k *prep, gloin CardID) kop.FilterID {
	t.Helper()
	cd := &k.t.Cards[gloin]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigSpellCast {
			continue
		}
		for f := a.Trig.Filter; f != 0; f = k.t.Filters[f].Or {
			if k.t.Filters[f].Pred == kop.PredHistoric {
				return a.Trig.Filter
			}
		}
		t.Fatal("Glóin's SpellCast trigger carries no Historic filter")
	}
	t.Fatal("Glóin has no SpellCast trigger")
	return 0
}

// faceDownCast pushes the hand card o's face-down cast onto the stack.
func faceDownCast(t *testing.T, sc *scen, o Obj) {
	t.Helper()
	d := sc.s.Step()
	var cast Action
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && a.Source() == TCode(o) && a.Mode()&castFaceDown != 0 {
			cast = a
			break
		}
	}
	if cast == 0 {
		d = sc.s.Step()
		t.Fatalf("face-down cast not offered; opts %v", d.Options[:d.N])
	}
	sc.s.Apply(cast)
	for i := 0; i < 20; i++ {
		d = sc.s.Step()
		if ob := sc.s.objs[o]; ob.zone == kop.ZoneStack && ob.slot == slotFaceDownSpell {
			return
		}
		if d.Kind == DkPriority && sc.s.ctl == ctlPriority {
			t.Fatalf("the face-down cast never reached the stack: kind %d ctl %d", d.Kind, sc.s.ctl)
		}
		sc.s.Apply(d.Options[0])
	}
	t.Fatal("the face-down cast never reached the stack")
}

// TestFaceDownSpellIsNotHistoric: the printed card is historic, but the
// face-down spell on the stack matches Glóin's Historic filter neither
// directly (CR 708.4) nor when the cast fires the trigger, so nothing is
// queued.
func TestFaceDownSpellIsNotHistoric(t *testing.T) {
	sc, akCard, gloin, ak := historicScen(t)
	f := historicTrigFilter(t, sc.k, sc.s.objs[gloin].card)
	if !sc.s.matchCardAt(sc.k, f, akCard, 0, 0, NoObj) {
		t.Fatal("Akroma's printed card does not match Glóin's Historic filter")
	}
	faceDownCast(t, sc, ak)
	if sc.s.matchCardAt(sc.k, f, akCard, 0, 0, ak) {
		t.Error("the face-down spell matched the Historic filter (CR 708.4)")
	}
	if sc.s.npend != 0 {
		p := &sc.s.pend[0]
		t.Fatalf("%d triggers queued for the face-down cast, want none (listener obj %d ability %d)", sc.s.npend, p.obj, p.ab)
	}
	if sc.s.sp != 1 || sc.s.stack[0].obj != ak {
		// the trigger placed on the stack rides it as an entry beside the
		// spell (the kfuzz ref/kernel NStack divergence)
		t.Fatalf("%d stack entries, want only the face-down spell: top is kind %d obj %d", sc.s.sp, sc.s.stack[sc.s.sp-1].kind, sc.s.stack[sc.s.sp-1].obj)
	}
}
