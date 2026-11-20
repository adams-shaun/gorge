package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCastTargetsAnyBattlefieldOnly mirrors gorge's
// TestValidSAIsTargetingValidIsBattlefieldOnly: Tf3OrTargets reads
// `IsTargeting Valid Creature.YouCtrl` (Danitha, Sword of Hope) over
// battlefield targets only (Forge's getDefinedObjects `Valid ` form; CR 109.2,
// 110.1). A creature card in the listener's graveyard is not "a creature you
// control".
func TestCastTargetsAnyBattlefieldOnly(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Downwind Ambusher", Count: 2}, {Name: "Plains", Count: 8}})
	creature := id["Downwind Ambusher"]
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)

	src := sc.perm(creature, 0)
	l := trigListener{obj: src, sl: sc.slot(src), ctrl: 0}
	var e stackEntry
	e.nt = 1

	mine := sc.perm(creature, 0)
	e.tgt[0] = TCode(mine)
	if !sc.s.castTargetsAny(sc.k, f, &e, l) {
		t.Fatal("a creature permanent the listener controls did not match")
	}
	grave := sc.obj(creature, 0)
	sc.s.objs[grave].zone = kop.ZoneGraveyard
	e.tgt[0] = TCode(grave)
	if sc.s.castTargetsAny(sc.k, f, &e, l) {
		t.Fatal("a creature card in the listener's graveyard matched Valid Creature.YouCtrl")
	}
}
