package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChooseTypeDepartedSourceKeepsChosen: a trigger whose source was
// destroyed in response still reads the source's last chosen creature type
// when it resolves (CR 113.7a, 608.2b, 608.2h). Herald's Horn's may-reveal
// asks over RevealValid$ Creature.ChosenType after Abrade destroyed the Horn;
// the trigger's own entry carries the triggering object's LKI, not the
// source's, so the choice comes from the source's departure snapshot.
func TestChooseTypeDepartedSourceKeepsChosen(t *testing.T) {
	sc, id, stone, bear, _ := gatheringStoneScen(t)
	sc.s.perms[sc.slot(stone)].chosen = objWord(bear)
	e := stackEntry{kind: seTrigger, obj: stone, srcVer: sc.s.objs[stone].ver, card: id["Gathering Stone"], ctrl: 0}
	sc.s.sp = 1
	sc.s.stack[0] = e
	sc.s.removePerm(sc.k, sc.slot(stone), kop.ZoneGraveyard, false)
	if got := sc.s.objs[stone].zone; got != kop.ZoneGraveyard {
		t.Fatalf("precondition: source is in zone %d, want the graveyard", got)
	}
	i := sc.s.srcDepartIdx(&e)
	if i == 0 {
		t.Fatal("precondition: the departed source has no departure snapshot")
	}
	if got := sc.s.lk(i).chosen; got != bear {
		t.Fatalf("precondition: departure snapshot chose %d, want Bear %d", got, bear)
	}
	sc.s.res = resCtx{e: e, active: true}
	if got := sc.s.srcChosen(NoSlot, stone); got != bear {
		t.Fatalf("srcChosen after the source left = %d, want the last chosen type %d", got, bear)
	}
	// The trigger's may-reveal still poses the ask: the top card is a Bear,
	// the type the departed Horn chose.
	pl := &sc.s.pl[0]
	sc.s.objs[pl.lib[int(pl.libLo)]].card = id["Grizzly Bears"]
	in := &kop.Instr{Op: kop.OpPeek, Obj: kop.RefController, A: kop.Const(1), C: kop.PeekMayReveal, F: kop.FilterID(4)}
	if sc.s.peekMayReveal(sc.k, in, 0) {
		t.Fatal("peekMayReveal skipped the ask after the source left; the chosen type must survive as LKI")
	}
}
