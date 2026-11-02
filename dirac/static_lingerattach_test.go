package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLingerStickyAttachment: a host that leaves the battlefield still shows
// its Aura and Equipment attached in the same priority window (gorge's
// projection reads the host until the next state-based pass), and the pass
// unattaches them. Covers both readers of the linger side table.
func TestLingerStickyAttachment(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{
		{Name: "Witness Protection", Count: 1},
		{Name: "Goldvein Pick", Count: 1},
		{Name: "Savannah Lions", Count: 1},
		{Name: "Plains", Count: 4},
	})
	s := sc.s
	if !s.g.on {
		t.Fatal("scenario table is not generated: linger is never recorded")
	}
	host := sc.perm(id["Savannah Lions"], 1)
	aura := sc.perm(id["Witness Protection"], 0)
	equip := sc.perm(id["Goldvein Pick"], 1)
	hs := sc.slot(host)
	s.perms[sc.slot(aura)].attached = hs
	s.perms[sc.slot(equip)].attached = hs
	s.leave(sc.k, hs, kop.ZoneGraveyard, false)
	if !s.g.lingerM.has(sc.slot(aura)) || !s.g.lingerM.has(sc.slot(equip)) {
		t.Fatalf("leaving host recorded linger slots %x, want the Aura and the Equipment", s.g.lingerM)
	}
	var p Projection
	s.Project(&p)
	if p.Objs[aura].Attached != host || p.Objs[equip].Attached != host {
		t.Fatalf("same-window projection: Aura attached to %d, Equipment to %d, want host %d",
			p.Objs[aura].Attached, p.Objs[equip].Attached, host)
	}
	if _, a, _ := s.FeatObjLinks(aura); a != host {
		t.Fatalf("same-window feature view: Aura attached to %d, want host %d", a, host)
	}
	if _, a, _ := s.FeatObjLinks(equip); a != host {
		t.Fatalf("same-window feature view: Equipment attached to %d, want host %d", a, host)
	}
	s.sba(sc.k)
	if s.g.lingerM.any() {
		t.Fatalf("the SBA pass left linger slots %x", s.g.lingerM)
	}
	s.Project(&p)
	if p.Objs[aura].Attached != NoObj || p.Objs[equip].Attached != NoObj {
		t.Fatalf("after the SBA pass: Aura shows host %d, Equipment shows host %d, want unattached",
			p.Objs[aura].Attached, p.Objs[equip].Attached)
	}
}

// TestLingerRecordsReuse: the record table is bounded, updates a slot that is
// already live, reuses a record the mask no longer names, and stalls (rather
// than truncating) only when every record is live. Slot 0 is the regression:
// empty records alias it, so the table must count the records written.
func TestLingerRecordsReuse(t *testing.T) {
	sc, _ := newObjScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	s := sc.s
	for i := Slot(0); i < maxLinger; i++ {
		s.lingerPut(i, Obj(i+1))
	}
	if s.g.nlinger != maxLinger {
		t.Fatalf("filled %d records, want %d", s.g.nlinger, maxLinger)
	}
	s.lingerPut(1, 77)
	if s.g.nlinger != maxLinger || s.lingerObj(1) != 77 {
		t.Fatalf("update: nlinger %d, slot 1 host %d; want %d, 77", s.g.nlinger, s.lingerObj(1), maxLinger)
	}
	s.g.lingerM.clr(0)
	s.lingerPut(20, 99)
	if s.g.nlinger != maxLinger || s.lingerObj(20) != 99 || s.lingerObj(0) != NoObj {
		t.Fatalf("reuse: nlinger %d, slot 20 host %d, slot 0 host %d; want %d, 99, %d",
			s.g.nlinger, s.lingerObj(20), s.lingerObj(0), maxLinger, NoObj)
	}
	s.lingerPut(30, 100)
	if s.g.stalled != StallLinger {
		t.Fatalf("overflow: stalled %d, want %d", s.g.stalled, StallLinger)
	}
}
