package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDepartedSourcePowerResolvingValue: an op operand reading its own
// source's power (Threefold Thunderhulk's CreateToken A=PowerOf(Self), "create
// Gnomes equal to its power") after the source was sacrificed in response
// reads its last known power (CR 608.2h), as gorge's Count$CardPower does.
// Before, resolve's value read the card's printed power, and the Thunderhulk
// (0/0 with three +1/+1 counters) made no Gnomes. The FDN Burglar Rat (1/1)
// stands in: the read is the same for any source, and its printed 1 differs
// from its last known 4.
func TestDepartedSourcePowerResolvingValue(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Burglar Rat", Count: 1}, {Name: "Swamp", Count: 8}})
	rat := sc.perm(id["Burglar Rat"], 0)
	sc.s.perms[sc.slot(rat)].setCtrN(sc.s, int(kop.CtrP1P1), 3)
	sc.s.res.e = stackEntry{kind: seTrigger, obj: rat, srcVer: sc.s.objs[rat].ver, card: sc.s.objs[rat].card, ab: 0, ctrl: 0, trigObj: NoObj}
	if p := sc.s.value(sc.k, kop.ValueRef{K: kop.VSelfPower}); p != 4 {
		t.Fatalf("precondition: the Rat's power on the battlefield %d, want 4", p)
	}
	sc.s.leave(sc.k, sc.slot(rat), kop.ZoneGraveyard, false)
	if z, printed := sc.s.objs[rat].zone, sc.k.cards[sc.s.objs[rat].card].power; z != kop.ZoneGraveyard || printed != 1 {
		t.Fatalf("precondition: the Rat in zone %d with printed power %d, want the graveyard and 1", z, printed)
	}
	if p := sc.s.value(sc.k, kop.ValueRef{K: kop.VSelfPower}); p != 4 {
		t.Fatalf("the Rat's power after it left: %d, want its last known 4", p)
	}
}
