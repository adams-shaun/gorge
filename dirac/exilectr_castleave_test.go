package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExileCounterCastLeaveDrops: a card cast out of exile on Dauthi
// Voidwalker's grant leaves exile through the cast push (cast.go's pushBuild
// writes zone and bumps ver directly, never placeFromK), and a counterspell's
// "exile it instead of putting it into its owner's graveyard" then runs
// placeFromK with ZoneExile — a move the store's leave-clear skips. The exile
// it makes is a new object (CR 400.7), so the record the first exile stamped
// must not answer for it: no rider runs on that move, and the new exile
// object carries no VOID counter until one is put there again.
func TestExileCounterCastLeaveDrops(t *testing.T) {
	sc, id := exileCtrScen(t)
	s, k := sc.s, sc.k
	sc.perm(id["Dauthi Voidwalker"], 0)
	bear := sc.perm(id["Grizzly Bears"], 1)
	if z := s.objs[bear].zone; z != kop.ZoneBattlefield {
		t.Fatalf("precondition: bear is in zone %d, want the battlefield", z)
	}

	// Dauthi exiles the dying bear with a VOID counter.
	s.leave(k, sc.slot(bear), kop.ZoneGraveyard, false)
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("the dying bear is in zone %d, want exile", z)
	}
	if n := s.exileCounters(bear, kop.CtrVoid); n != 1 {
		t.Fatalf("precondition: the exiled bear has %d VOID counters, want 1", n)
	}

	// Dauthi's effect grants a play of the chosen card: the grant rides the
	// remembered set (OpMayPlay over Exile), as the built body does.
	s.g.rem[0], s.g.nrem = bear, 1
	in := kop.Instr{
		Op:  kop.OpMayPlay,
		Obj: kop.RefRemembered,
		C:   int16(kop.ZoneExile),
		A:   kop.Const(kop.DurEOT),
		B:   kop.Const(kop.MayPlayFree | kop.MayPlayAnyType),
	}
	s.mayPlayRemembered(k, &in, 0)
	if s.g.nmay != 1 || s.g.may[0].obj != bear {
		t.Fatalf("precondition: %d permissions (%v), want the chosen bear's play permission", s.g.nmay, s.g.may[:s.g.nmay])
	}

	// Cast it out of exile: the real cast path, not a placeFromK move. The
	// push commits when the next decision is built (payment done), so the
	// step after the cast is what moves the card off exile.
	cast := false
	d := s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(bear)) {
			s.Apply(a)
			cast = true
			break
		}
	}
	if !cast {
		t.Fatalf("precondition: the granted cast was not offered: %v", d.Options[:d.N])
	}
	s.Step()
	if s.Stalled() {
		t.Fatalf("the cast stalled: %d", s.g.stalled)
	}
	if z := s.objs[bear].zone; z != kop.ZoneStack {
		t.Fatalf("the cast card is in zone %d, want the stack", z)
	}
	ver := s.objs[bear].ver
	if n := s.exileCounters(bear, kop.CtrVoid); n != 0 {
		t.Fatalf("the spell on the stack still reports %d exile VOID counters", n)
	}

	// A counterspell exiles it instead of the graveyard: placeFromK with
	// ZoneExile, the move the leave-clear skips.
	s.placeFromK(k, bear, kop.ZoneExile, false, kop.ZoneStack, false, false)
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("the countered spell is in zone %d, want exile", z)
	}
	if s.objs[bear].ver == ver {
		t.Fatal("precondition: exiling the spell did not make a new object (version unchanged)")
	}
	if n := s.exileCounters(bear, kop.CtrVoid); n != 0 {
		t.Fatalf("the re-exiled spell reports %d VOID counters, want 0: the old exile's record outlived the cast", n)
	}
}
