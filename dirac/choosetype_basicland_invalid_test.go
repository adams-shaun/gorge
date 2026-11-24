package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
)

// fnvLabel is the kernel's option value for a label: 0x8000 | FNV-1a(label) & 0x7fff.
func fnvLabel(s string) uint16 {
	v := uint32(2166136261)
	for j := 0; j < len(s); j++ {
		v ^= uint32(s[j])
		v *= 16777619
	}
	return 0x8000 | uint16(v&0x7fff)
}

// TestChooseTypeBasicLandInvalidTypesAsk: an as-enters Basic Land ask whose
// OpChooseType carries A = the InvalidTypes$ mask offers only the unexcluded
// labels (gorge effects.TypeChoiceLabels), and the answer is stored as the
// chosen subtype.
func TestChooseTypeBasicLandInvalidTypesAsk(t *testing.T) {
	run := func(mask int) (*Decision, *scen, Obj, *Table) {
		set := cardsettest.Set(t, "choosebasicenter")
		deck := []cardset.Entry{{Name: "Realmwright", Count: 4}, {Name: "Island", Count: 18}, {Name: "Mountain", Count: 18}}
		tbl, da, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		if mask != 0 {
			n := 0
			cd := &tbl.Cards[id["Realmwright"]]
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
					continue
				}
				for j := range a.Prog {
					if isBasicLandChoose(&a.Prog[j]) {
						a.Prog[j].A = kop.Const(mask)
						n++
					}
				}
			}
			if n != 1 {
				t.Fatalf("found %d as-enters basic land chooses to mask, want 1", n)
			}
		}
		sc := newScenTableLib(t, tbl, id["Island"], Config{})
		sc.perm(id["Island"], 0)
		rw := sc.hand(id["Realmwright"], 0)
		sc.do(t, AkCast, TCode(rw), scAny)
		var ask *Decision
		for i := 0; i < 40 && ask == nil; i++ {
			d := sc.s.Step()
			switch {
			case d.Kind == DkChoose:
				ask = &d
			case d.Kind == DkPriority:
				sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			default:
				t.Fatalf("unexpected decision kind %d before the ask", d.Kind)
			}
		}
		if ask == nil || ask.Sub != ChGenType {
			t.Fatalf("Realmwright posed no type ask: %+v", ask)
		}
		return ask, sc, rw, tbl
	}

	// Control: the unmodified table offers all five.
	if ask, _, _, _ := run(0); int(ask.N) != len(basicLandLabelH) {
		t.Fatalf("unmasked ask offers %d, want %d", ask.N, len(basicLandLabelH))
	}

	ask, sc, rw, tbl := run(1<<0 | 1<<2 | 1<<3)
	if ask.N != 2 {
		t.Fatalf("masked ask offers %d options, want 2 (Island, Swamp)", ask.N)
	}
	got := map[uint16]Action{}
	for i := 0; i < int(ask.N); i++ {
		a := ask.Options[i]
		got[uint16(a.Target())<<8|uint16(a.Mode())] = a
	}
	labels := effects.TypeChoiceLabels("Basic Land", "", "Mountain,Forest,Plains")
	if len(labels) != 2 {
		t.Fatalf("gorge offers %v, want two labels", labels)
	}
	for i := 0; i < int(ask.N); i++ {
		a := ask.Options[i]
		if v := uint16(a.Target())<<8 | uint16(a.Mode()); v != fnvLabel(labels[i]) {
			t.Errorf("option %d = %#x, gorge's %s hashes to %#x", i, v, labels[i], fnvLabel(labels[i]))
		}
	}
	if _, ok := got[basicLandLabelH[1]]; !ok {
		t.Error("Island not offered")
	}
	swamp, ok := got[basicLandLabelH[4]]
	if !ok {
		t.Fatal("Swamp not offered")
	}

	sub := -1
	for i, n := range tbl.Subtypes {
		if n == "Swamp" {
			sub = i
		}
	}
	if sub < 0 {
		t.Fatalf("subtypes %v hold no Swamp", tbl.Subtypes)
	}
	sc.s.Apply(swamp)
	for i := 0; i < 10 && sc.slot(rw) == NoSlot; i++ {
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d while Realmwright resolves", d.Kind)
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	sl := sc.slot(rw)
	if sl == NoSlot {
		t.Fatal("Realmwright never entered")
	}
	if g := sc.s.perms[sl].chosen; g != objWord(uint8(sub)) {
		t.Errorf("Realmwright chose subtype %d, want Swamp (%d)", g, sub)
	}
}
