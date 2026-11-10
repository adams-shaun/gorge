package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRestrictedFixedColourPairView: Tablet of Discovery ({T}: Add {R}; {T}:
// Add {R}{R}, spell-only for instants and sorceries) is a restricted pair unit
// whose larger alternative is two mana of {R} alone (wideN 2, wideCol R), not
// of any colour: restrSrcAvail counts it as two {R} with no one-colour
// ("any one colour of my choosing") group, which would let a cost take {G}{G}
// from it (CR 106.6, 601.2g-h).
func TestRestrictedFixedColourPairView(t *testing.T) {
	set := cardsettest.Set(t, "restrictedfixedpair")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	var tablet, mountain, spell CardID
	found := 0
	for c := range tbl.Cards {
		switch tbl.Cards[c].Name {
		case "Tablet of Discovery":
			tablet = CardID(c)
			found |= 1
		case "Mountain":
			mountain = CardID(c)
			found |= 2
		}
		if spell == 0 && tbl.Cards[c].Types&kop.TInstant != 0 && tbl.Cards[c].Types&kop.TLand == 0 {
			spell = CardID(c)
			found |= 4
		}
	}
	if found != 7 {
		t.Fatalf("restrictedfixedpair table lacks Tablet, Mountain or an instant (found mask %d)", found)
	}
	sc := newScenTableLib(t, tbl, mountain, Config{})
	g := sc.k.gen
	rec := g.restrRecOf(tablet)
	if rec == nil || rec.kind != rkWiden {
		t.Fatalf("Tablet's restricted record = %+v, want a widened source", rec)
	}
	v := &g.rs.views[rec.view]
	if v.wideN != 2 || v.wideCol != kop.ColR {
		t.Fatalf("Tablet's view wideN %d wideCol %v, want 2 of {R}", v.wideN, v.wideCol)
	}
	b := sc.perm(tablet, 0)
	var base, got avail
	d := payDesc{class: pdSpell, card: spell, src: NoSlot}
	if _, k, _ := sc.s.restrView(sc.k, 0, sc.slot(b), sc.s.manaPCGen(sc.k, sc.slot(b)), &d); k != rkWiden {
		t.Fatalf("an instant payment does not admit Tablet's restricted ability (kind %d): the test reads nothing", k)
	}
	sc.s.availFor(sc.k, 0, &base)
	if base.total != 1 {
		t.Fatalf("Tablet alone plans %d mana, want its census {R}", base.total)
	}
	got, _, _ = sc.s.restrSrcAvail(sc.k, 0, d, &base, NoSlot)
	if got.total != 2 || got.nOne != 0 {
		t.Errorf("restrSrcAvail: %d mana and %d one-colour groups, want 2 and none", got.total, got.nOne)
	}
}
