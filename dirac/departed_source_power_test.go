package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDepartedSourcePowerResolvingValue: an op operand reading its own
// source's power (Threefold Thunderhulk's CreateToken A=PowerOf(Self), "create
// Gnomes equal to its power") after the source was sacrificed in response
// reads its last known power, 3 with its three +1/+1 counters (CR 608.2h),
// as gorge's Count$CardPower does. Before, resolve's value read the card's
// printed 0 and the trigger made no Gnomes.
func TestDepartedSourcePowerResolvingValue(t *testing.T) {
	set := cardsettest.Synth(t, "std-lci")
	if i, ok := set.Lookup("Threefold Thunderhulk"); !ok || !set.Supported()[i] {
		t.Fatal("precondition: Threefold Thunderhulk is not supported in the std-lci table")
	}
	deck := []cardset.Entry{{Name: "Threefold Thunderhulk", Count: 1}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	th := sc.perm(id["Threefold Thunderhulk"], 0)
	sc.s.perms[sc.slot(th)].setCtrN(sc.s, int(kop.CtrP1P1), 3)
	sc.s.res.e = stackEntry{kind: seTrigger, obj: th, srcVer: sc.s.objs[th].ver, card: sc.s.objs[th].card, ab: 1, ctrl: 0, trigObj: NoObj}
	if p := sc.s.value(sc.k, kop.ValueRef{K: kop.VSelfPower}); p != 3 {
		t.Fatalf("precondition: Thunderhulk's power on the battlefield %d, want 3", p)
	}
	sc.s.leave(sc.k, sc.slot(th), kop.ZoneGraveyard, false)
	if z := sc.s.objs[th].zone; z != kop.ZoneGraveyard {
		t.Fatalf("precondition: Thunderhulk in zone %d, want the graveyard", z)
	}
	if p := sc.s.value(sc.k, kop.ValueRef{K: kop.VSelfPower}); p != 3 {
		t.Fatalf("Thunderhulk's power after it left: %d, want its last known 3", p)
	}
}
