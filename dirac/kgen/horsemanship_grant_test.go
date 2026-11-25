package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// horsemanshipGrantLower lowers an inline script (no corpus registry needed).
func horsemanshipGrantLower(t *testing.T, src string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("horsemanship_grant_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerPool(p).cards[0]
}

// TestGrantedHorsemanshipLowers: a Pump / PumpAll KW$ Horsemanship until end
// of turn (Riding Red Hare, Merry-Go-Round's PumpAll) lowers as an ExtKwMark
// carrying kop.KwMarkHorsemanship, beside the pump it grants with. A
// Continuous static's AddKeyword$ Horsemanship (Sun Quan, Lord of Wu) lowers
// to Static.Evasion EvHorsemanship (evasionGranted reads it).
func TestGrantedHorsemanshipLowers(t *testing.T) {
	const head = "Name:Probe\nManaCost:1 W\nTypes:Instant\n"
	for _, c := range []struct{ name, src string }{
		{"Pump (Riding Red Hare)", head + "A:SP$ Pump | ValidTgts$ Creature | NumAtt$ +3 | NumDef$ +3 | KW$ Horsemanship\n"},
		{"PumpAll (Merry-Go-Round)", head + "A:SP$ PumpAll | ValidCards$ Creature.powerLE2+YouCtrl | KW$ Horsemanship\n"},
	} {
		lc := horsemanshipGrantLower(t, c.src)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refused: %v", c.name, lc.Refusals)
			continue
		}
		marks := 0
		for _, in := range lc.Abilities[0].Prog {
			if in.Op == "KwMark" {
				marks |= in.C
			}
		}
		if marks != kop.KwMarkHorsemanship {
			t.Errorf("%s: KwMark C = %d, want kop.KwMarkHorsemanship (%d)", c.name, marks, kop.KwMarkHorsemanship)
		}
	}
	static := horsemanshipGrantLower(t, "Name:Probe\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n"+
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ Horsemanship\n")
	if len(static.Refusals) != 0 {
		t.Fatalf("a Horsemanship static refused: %v", static.Refusals)
	}
	if len(static.Statics) != 1 || static.Statics[0].Evasion != kop.EvHorsemanship {
		t.Errorf("a Horsemanship static: statics %+v, want one with Evasion EvHorsemanship", static.Statics)
	}
}

// TestRidingRedHareLowers: the corpus card lowers and builds with no refusal.
func TestRidingRedHareLowers(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Riding Red Hare"})
	if len(lw.cards) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(lw.cards))
	}
	for _, r := range lw.cards[0].Refusals {
		t.Errorf("refused: %s", r.Reason)
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
	}
}
