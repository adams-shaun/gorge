package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLeaveTogetherMovesSparedUmbraAura: in a multi-target destroy an umbra
// Aura that is itself a target but is spared by a non-umbra replacement (here
// indestructible) is not leaving on its own. The bearer's umbra armor still
// destroys it (gorge replaceUmbraArmor's MoveZone consults neither
// indestructible nor a shield), so leaveTogether must keep a spared Aura out
// of the co-victim batch and let the bearer's save move it.
func TestLeaveTogetherMovesSparedUmbraAura(t *testing.T) {
	sc, id := umbraScen(t)
	s, k := sc.s, sc.k

	bearer := sc.perm(id["Hill Giant"], 1)
	aura := sc.wear(id["Bear Umbra"], bearer)
	bs, as := sc.slot(bearer), sc.slot(aura)
	// A "permanents you control gain indestructible" effect on the Aura.
	s.perms[as].eotKw |= kop.KwIndestructible
	if s.kw(k, as)&kop.KwIndestructible == 0 {
		t.Fatal("precondition: the Aura is not indestructible")
	}
	if got := s.umbraAura(k, bs, slotMask{}); got != as {
		t.Fatalf("precondition: the bearer's umbra Aura is %d, want %d", got, as)
	}
	if !s.destroySpared(k, as, false) {
		t.Fatal("precondition: the indestructible Aura is not spared without umbra")
	}
	s.perms[bs].damage = 2

	// A destroy naming the bearer and the Aura together (the first target and
	// one extra target of the same target slot).
	s.res.e.ctrl = 0
	s.res.e.tgt[0], s.res.e.tver[0] = TCode(bearer), s.objs[bearer].ver
	xt := s.xtMut(&s.res.e)
	if xt == nil {
		t.Fatal("setup: no extra-target pool entry")
	}
	*xt = xTargets{n: 1}
	xt.slot[0], xt.code[0], xt.ver[0] = 0, TCode(aura), s.objs[aura].ver
	s.res.bad, s.res.xbad = 0, 0

	if !s.leaveTogether(k, &kop.Instr{Op: kop.OpDestroy, Obj: kop.RefTarget0}, 0) {
		t.Fatal("leaveTogether did not take the two-target destroy")
	}
	if !sc.onBF(bearer) {
		t.Fatal("the bearer died although its umbra Aura answered")
	}
	if s.perms[bs].damage != 0 {
		t.Fatalf("the saved bearer has damage %d, want 0", s.perms[bs].damage)
	}
	if !sc.inGrave(aura) {
		t.Fatal("the spared umbra Aura stayed on the battlefield; the bearer's umbra armor destroys it (gorge replaceUmbraArmor)")
	}
}
