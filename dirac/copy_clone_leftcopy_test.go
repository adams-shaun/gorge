package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A copy that left the battlefield keeps the copied card's lines (CR 113.7a,
// 603.10a; gorge Object.LeftCopyFace): a dies trigger of the copied card asks
// its CR 603.5 optional question even though the reverted printed card no
// longer carries the line. Necropede's "you may put a -1/-1 counter" dies
// trigger on a True Polymorph copy that died is the lockstep pair this pins
// (mbc_ruthless_deathfang_counter_power).
func TestCloneTrigMovedLeftCopyAsks(t *testing.T) {
	t.Parallel()
	if len(v.Table.Cards) < 2 {
		t.Fatal("precondition: the vanilla table has two cards")
	}
	printed, copy := CardID(0), CardID(1) // the source died as a copy of `copy`
	k := &prep{t: v.Table, gen: &genPrep{anyCloneCopy: true}}
	prepCold(k.gen, &k.cold)
	k.cold.size()
	s := new(State)
	s.layOut(&k.cold)
	if n := len(coldLeftCopy.of(s)); n != MaxObj {
		t.Fatalf("precondition: the leftCopy region is laid out for the table (len %d)", n)
	}
	const o = Obj(3)
	s.objs[o].card = printed
	s.objs[o].zone = kop.ZoneGraveyard
	e := stackEntry{kind: seTrigger, card: copy, obj: o}
	if !s.cloneTrigMoved(k, &e) {
		t.Fatal("precondition: with no departed copy face the source counts as moved")
	}
	if got, ok := s.leftCopyCard(o); ok || got != 0 {
		t.Fatalf("precondition: no departed copy card yet, got %d (set %v)", got, ok)
	}
	// the source left the battlefield as a copy of the ability's card: gorge
	// finds the line on the departed copy's face and asks
	s.leftCopySet(o, copy)
	if got, ok := s.leftCopyCard(o); !ok || got != copy {
		t.Fatalf("precondition: leftCopy card = %d (set %v), want %d", got, ok, copy)
	}
	if s.cloneTrigMoved(k, &e) {
		t.Fatal("the departed copy's own trigger line is asked")
	}
	// a later copy face is the only one the departed basis holds: the line of
	// an older copy face is not found
	s.leftCopySet(o, printed)
	if !s.cloneTrigMoved(k, &e) {
		t.Fatal("the line of a superseded copy face is not asked")
	}
	// table card 0 is a real card, so "left as a copy of card 0" is a set
	// basis, not the none marker: its lines are asked, and a source that never
	// left as a copy counts as moved even when the trigger's card is card 0
	// (gorge's nil LeftCopyFace asks nothing)
	const o2 = Obj(4)
	s.objs[o2].card = copy
	s.objs[o2].zone = kop.ZoneGraveyard
	e2 := stackEntry{kind: seTrigger, card: printed, obj: o2}
	if !s.cloneTrigMoved(k, &e2) {
		t.Fatal("no departed copy face: the source counts as moved even with card 0")
	}
	s.leftCopySet(o2, printed)
	if got, ok := s.leftCopyCard(o2); !ok || got != printed {
		t.Fatalf("precondition: leftCopy card = %d (set %v), want %d", got, ok, printed)
	}
	if s.cloneTrigMoved(k, &e2) {
		t.Fatal("the departed copy of card 0's own trigger line is asked")
	}
}
