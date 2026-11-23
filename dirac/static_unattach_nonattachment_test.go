package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUnattachNonattachmentWithoutGrant: a noncreature permanent that is no
// attachment type, was never printed as one and holds no live Equip grant is
// gorge's attachedKindIllegal "non-attachment permanent becomes unattached"
// arm (CR 704.5p), so sbaAttachedCreature detaches it. Both with
// gainsOfAttach off and with it on (the grant walk finding no Equip source)
// the recipient has no grant to carve it out.
func TestUnattachNonattachmentWithoutGrant(t *testing.T) {
	set := cardsettest.Set(t, "marvingainsof")
	deck := []cardset.Entry{{Name: "Marvin, Murderous Mimic", Count: 20}}
	other := []cardset.Entry{{Name: "Prodigal Sorcerer", Count: 20}}
	tbl, _, _, err := set.Matchup(deck, other)
	if err != nil {
		t.Fatal(err)
	}
	var marvin, sorcerer CardID
	for i := range tbl.Cards {
		switch tbl.Cards[i].Name {
		case "Marvin, Murderous Mimic":
			marvin = CardID(i)
		case "Prodigal Sorcerer":
			sorcerer = CardID(i)
		}
	}
	sc := newScenTableLib(t, tbl, marvin, Config{NoElide: true})
	m := sc.slot(sc.perm(marvin, 0))
	h := sc.slot(sc.perm(sorcerer, 0))
	if sc.k.gainsOfAttach {
		t.Fatal("precondition: the table has no Equip to grant, gainsOfAttach should be off")
	}
	if tbl.Cards[sc.s.cardAt(m)].Types&(kop.TEquipment|kop.TAura) != 0 {
		t.Fatal("precondition: the recipient is printed as an attachment")
	}
	if sc.s.grantedEquipKeyword(sc.k, m) {
		t.Fatal("precondition: the recipient holds a live Equip grant")
	}
	for _, on := range []bool{false, true} {
		sc.k.gainsOfAttach = on // on: the grant walk runs and finds no Equip source
		sc.s.perms[m].attached = h
		// Types 0: a noncreature, non-Equipment, non-Aura permanent.
		if !sc.s.sbaAttachedCreature(sc.k, m, 0) {
			t.Fatalf("gainsOfAttach=%v: a noncreature non-attachment with no Equip grant stayed attached, want it unattached", on)
		}
		if sc.s.perms[m].attached != NoSlot {
			t.Fatalf("gainsOfAttach=%v: sbaAttachedCreature reported a detach but left the attachment", on)
		}
	}
}
