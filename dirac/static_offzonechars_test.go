package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestOffZoneCharsProject: the off-battlefield projection of the layer-4
// override grants (offZoneChars). The lockstep compares battlefield
// characteristics only (gorge's type filters read the layer-4 table, built
// from battlefield objects), so this is the projection's guard.
//
// Maskwood Nexus gives its controller's creature cards outside the battlefield
// every creature type, and no one else's, and no noncreature's. Rootpath
// Purifier makes its controller's land cards in the library basic, and only
// there (its Library zone).
func TestOffZoneCharsProject(t *testing.T) {
	deck := []cardset.Entry{{Name: "Maskwood Nexus", Count: 1}, {Name: "Rootpath Purifier", Count: 1},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Goblin Matron", Count: 1}, {Name: "Lightning Bolt", Count: 1}, {Name: "Blood Crypt", Count: 1}, {Name: "Forest", Count: 10}}
	tbl, da, _, err := cardsettest.Set(t, "offzonegrants").Matchup(deck, deck)
	if err != nil {
		t.Skip(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	bears, bolt, crypt := id["Grizzly Bears"], id["Lightning Bolt"], id["Blood Crypt"]
	mine := sc.hand(bears, 0)
	theirs := sc.hand(bears, 1)
	spell := sc.hand(bolt, 0)
	grave := sc.grave(bears, 0, 1)[0]
	libMine := sc.obj(crypt, 0)
	sc.s.place(libMine, kop.ZoneLibrary, false)
	libTheirs := sc.obj(crypt, 1)
	sc.s.place(libTheirs, kop.ZoneLibrary, false)
	handLand := sc.hand(crypt, 0)

	// no grantor yet: nothing is projected
	before := OffZoneCharStats.Hits.Load()
	for _, o := range []Obj{mine, grave, libMine} {
		if _, _, _, _, ok := sc.s.FeatViewTypes(o); ok {
			t.Fatalf("object %d projected with no grantor", o)
		}
	}
	if OffZoneCharStats.Hits.Load() != before {
		t.Fatal("a hit counted with no grantor")
	}

	sc.perm(id["Maskwood Nexus"], 0)
	sc.perm(id["Rootpath Purifier"], 0)
	sc.s.refreshTy(sc.k)

	goblin := slices.Index(tbl.Subtypes, "Goblin")
	fold := func(o Obj) (kop.SubMask, bool) {
		ob := &sc.s.objs[o]
		_, subs, all := sc.s.offZoneChars(sc.k, ob.card, ob.owner(), o, sc.k.cards[ob.card].types, tbl.Cards[ob.card].Subtypes)
		return subs, all
	}
	for _, o := range []Obj{mine, grave} {
		if subs, all := fold(o); !all || !subs.Has(goblin) {
			t.Errorf("Maskwood: own creature card %d in zone %d: subs %v all %v, want every creature type", o, sc.s.objs[o].zone, subs, all)
		}
		// an all-creature-types grant alone leaves the line as printed (gorge ViewTypes)
		if _, _, _, _, ok := sc.s.FeatViewTypes(o); ok {
			t.Errorf("Maskwood: card %d projects a changed line", o)
		}
	}
	if _, all := fold(theirs); all {
		t.Error("Maskwood: the opponent's creature card in hand got every creature type")
	}
	if _, all := fold(spell); all {
		t.Error("Maskwood: an instant in hand got every creature type")
	}
	if ty, _, _, _, ok := sc.s.FeatViewTypes(libMine); !ok || ty&kop.TBasic == 0 || ty&kop.TLand == 0 {
		t.Errorf("Rootpath: own land card in the library types %#x ok %v, want a basic land", ty, ok)
	}
	if ty, _, _, _, ok := sc.s.FeatViewTypes(libTheirs); ok && ty&kop.TBasic != 0 {
		t.Error("Rootpath: the opponent's library land became basic")
	}
	if ty, _, _, _, ok := sc.s.FeatViewTypes(handLand); ok && ty&kop.TBasic != 0 {
		t.Error("Rootpath: a land card in hand became basic; the grant is battlefield and library only")
	}
	if OffZoneCharStats.Hits.Load() == before {
		t.Error("no hit counted for the projections that changed")
	}

	// the table without the override records reads as printed
	for i := range tbl.Cards {
		for j := range tbl.Cards[i].Abilities {
			if st := &tbl.Cards[i].Abilities[j].Static; st.Kind == kop.StatOffZoneGrant && st.Override != 0 {
				st.Override = 0
			}
		}
	}
	sc.s.refreshTy(sc.k)
	if _, all := fold(mine); all {
		t.Error("an off-zone grant with its Override cleared still grants")
	}
	if ty, _, _, _, ok := sc.s.FeatViewTypes(libMine); ok && ty&kop.TBasic != 0 {
		t.Error("an off-zone grant with its Override cleared still projects")
	}
}
