package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCountTgtEntryMatches holds countTgtEntryMatches to the one-level-nested
// read (gorge's TargetedByTarget$Valid): the OUTER entry (the resolving spell,
// or the spell being built) targets the INNER entry, and what is counted is the
// INNER entry's own targets matching the filter -- never the outer's, which is
// why the outer aims at the inner spell and the inner at the creature. The
// filter, the player-target skip, the multi-object extras and the build's entry
// are each pinned, so an op that walked the wrong entry, ignored F or indexed a
// player byte as an object fails here rather than silently pricing a spell.
func TestCountTgtEntryMatches(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Island", Count: 4}})
	lion := sc.perm(id["Savannah Lions"], 0)
	creature := kop.FilterID(len(sc.k.t.Filters))
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature})
	instant := kop.FilterID(len(sc.k.t.Filters))
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TInstant})

	// the inner spell: an instant on the stack targeting the lion.
	innerObj := sc.s.newToken(id["Savannah Lions"], 0)
	sc.s.place(innerObj, kop.ZoneStack, false)
	inner := stackEntry{kind: seSpell, obj: innerObj, ctrl: 0, card: id["Savannah Lions"], nt: 1}
	inner.tgt[0] = TCode(lion)
	sc.s.push(inner)
	innerIdx := int(sc.s.sp) - 1

	// the outer spell: targeting the inner spell, so its OWN targets match no
	// creature filter and a count over them would read zero.
	outerObj := sc.s.newToken(id["Savannah Lions"], 0)
	sc.s.place(outerObj, kop.ZoneStack, false)
	outer := stackEntry{kind: seSpell, obj: outerObj, ctrl: 0, card: id["Savannah Lions"], nt: 1}
	outer.tgt[0] = TCode(innerObj)
	sc.s.push(outer)
	outerIdx := int(sc.s.sp) - 1

	count := func(f kop.FilterID, e *stackEntry) int {
		return sc.s.countTgtEntryMatches(sc.k, f, evalCtx{e: e, ctrl: 0})
	}
	if got := count(creature, &sc.s.stack[outerIdx]); got != 1 {
		t.Fatalf("the inner spell's creature target counted %d, want 1", got)
	}
	if got := count(instant, &sc.s.stack[outerIdx]); got != 0 {
		t.Errorf("the inner target counted %d against an instant filter, want 0: F is not applied", got)
	}

	// A player target of the inner entry is skipped, not counted and not
	// indexed as an object (gorge's IsPlayer skip).
	e := &sc.s.stack[innerIdx]
	e.nt = 2
	e.tgt[1] = PlayerTarget0 + 1
	if got := count(creature, &sc.s.stack[outerIdx]); got != 1 {
		t.Errorf("with a player target added the count is %d, want 1", got)
	}
	e.nt = 1

	// The inner entry's multi-object extras are its targets too.
	other := sc.perm(id["Savannah Lions"], 0)
	ie := &sc.s.stack[innerIdx]
	if x := sc.s.xtMut(ie); x == nil {
		t.Fatal("no extra-target pool entry")
	} else {
		x.n = 1
		x.code[0] = TCode(other)
	}
	if got := count(creature, &sc.s.stack[outerIdx]); got != 2 {
		t.Errorf("the inner entry's extra target counted %d, want 2", got)
	}
	x := sc.s.xtOf(ie)
	x.n = 0
	ie.xi = 0

	// An inner entry with no targets counts nothing: this is what fails if the
	// walk reads the OUTER entry's own targets instead (its one target is a
	// spell, which no creature filter matches).
	ie.tgt[0] = NoTarget
	ie.nt = 0
	if got := count(creature, &sc.s.stack[outerIdx]); got != 0 {
		t.Errorf("an inner entry with no targets counted %d, want 0", got)
	}
	ie.nt = 1
	ie.tgt[0] = TCode(lion)

	// A stack-ability target byte names its entry as well as a spell object does.
	oe := &sc.s.stack[outerIdx]
	oe.tgt[0] = StackTarget0 + TCode(innerIdx)
	if got := count(creature, oe); got != 1 {
		t.Errorf("the inner entry named by a stack-ability code counted %d, want 1", got)
	}
	oe.tgt[0] = TCode(innerObj)

	// The offer gate and the CR 601.2c/h reprice: with no resolving entry, the
	// spell being built is the subject (gorge binds the cast's chosen targets).
	*sc.s.bld() = build{e: sc.s.stack[outerIdx]}
	sc.s.ctl = ctlBuild
	if got := count(creature, nil); got != 1 {
		t.Errorf("the build's own targets counted %d, want 1", got)
	}
	sc.s.ctl = ctlPriority
}
