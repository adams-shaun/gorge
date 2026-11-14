package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMaxPermBoundary: the board is MaxPerm slots, the
// high slot-mask word carries only MaxPerm%64 valid bits, and nothing at or
// past MaxPerm is ever set, by the helpers or by a full board.
func TestMaxPermBoundary(t *testing.T) {
	if !slotBit(MaxPerm - 1).has(MaxPerm - 1) {
		t.Fatalf("slotBit(%d) is empty", MaxPerm-1)
	}
	for _, sl := range []Slot{MaxPerm, MaxPerm + 1, 127, 128, NoSlot} {
		if b := slotBit(sl); b.any() {
			t.Fatalf("slotBit(%d) = %+v, want empty", sl, b)
		}
		var m slotMask
		m.set(sl)
		if m.any() || m.with(sl).any() {
			t.Fatalf("set/with(%d) touched the mask: %+v", sl, m)
		}
	}
	var f slotMask
	for sl := 0; sl < MaxPerm; sl++ {
		f.set(Slot(sl))
	}
	if !f.full() || f.hi&^slotLast != 0 || f.count() != MaxPerm || f.not().any() {
		t.Fatalf("full board at %d: full %v hi %#x count %d", MaxPerm, f.full(), f.hi, f.count())
	}
	if n := (slotMask{}).not(); !n.full() || n.hi&^slotLast != 0 || n.count() != MaxPerm {
		t.Fatalf("not(empty) = %+v, want exactly the %d board slots", n, MaxPerm)
	}
	sc := maxPermScen(t)
	if sc.s.occ.hi&^slotLast != 0 {
		t.Fatalf("a full board set bits past MaxPerm: hi %#x", sc.s.occ.hi)
	}
}

// maxPermScen is a board holding MaxPerm permanents (slots 0..MaxPerm-1,
// the second slot-mask word included) on the generated path.
func maxPermScen(t *testing.T) *scen {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path (prep.gen != nil)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	for i := 0; i < MaxPerm; i++ {
		sc.perm(v.GrizzlyBears, 0)
	}
	if got := sc.s.occ.count(); got != MaxPerm {
		t.Fatalf("board holds %d of %d permanents", got, MaxPerm)
	}
	if !sc.s.occ.has(64) || !sc.s.occ.has(MaxPerm-1) {
		t.Fatalf("slots 64 and %d not occupied", MaxPerm-1)
	}
	if sc.s.occ.has(MaxPerm) || sc.s.occ.has(NoSlot) {
		t.Fatalf("out-of-range slot %d or NoSlot reads as occupied", MaxPerm)
	}
	if sc.s.Stalled() {
		t.Fatalf("%d permanents stalled %q", MaxPerm, StallName(sc.s.StallReason()))
	}
	return sc
}

// TestMaxPerm128: the battlefield holds MaxPerm permanents (slots 0..MaxPerm-1, the
// second slot-mask word included), and the next one stalls loud with
// StallBoard instead of truncating. Both capacity draws are
// covered: a token creation (the nearCapacity gate at MaxPerm-4) and a land
// play at a full board (legal.go's occ.full() gate, the card-permanent path).
func TestMaxPerm128(t *testing.T) {
	t.Run("token", func(t *testing.T) {
		sc := maxPermScen(t)
		sc.s.res.e.ctrl = 0
		in := kop.Instr{Op: kop.OpCreateToken, Obj: kop.RefController, A: kop.Const(1), C: int16(v.GrizzlyBears)}
		sc.s.execGen(sc.k, &in)
		if got := sc.s.StallReason(); got != StallBoard {
			t.Fatalf("the %dth permanent: stall %q, want board", MaxPerm+1, StallName(got))
		}
	})

	t.Run("land", func(t *testing.T) {
		sc := maxPermScen(t)
		land := sc.hand(v.Forest, 0)
		sc.s.Apply(MakeAction(AkPlayLand, TCode(land), 0, 0))
		if got := sc.s.StallReason(); got != StallBoard {
			t.Fatalf("the %dth permanent (a land play): stall %q, want board", MaxPerm+1, StallName(got))
		}
		if sc.s.occ.count() != MaxPerm || sc.s.objs[land].zone != kop.ZoneHand {
			t.Fatalf("the full board changed: %d permanents, land zone %d", sc.s.occ.count(), sc.s.objs[land].zone)
		}
	})
}

// TestSlotMaskSecondWord covers the two-word paths on the second word alone
// (word 0 empty), where a naive many() would report one slot as two.
func TestSlotMaskSecondWord(t *testing.T) {
	if MaxPerm <= 64 {
		t.Skip("one-word mask")
	}
	var m slotMask
	m.set(64)
	if !m.any() || m.count() != 1 || m.many() || m.first() != 64 {
		t.Fatalf("slot 64 alone: any %v count %d many %v first %d", m.any(), m.count(), m.many(), m.first())
	}
	if m.has(0) || !m.has(64) || m.has(65) {
		t.Fatalf("membership around the word boundary is wrong")
	}
	m.set(65)
	if m.count() != 2 || !m.many() || m.first() != 64 {
		t.Fatalf("slots 64+65: count %d many %v first %d", m.count(), m.many(), m.first())
	}
	var one slotMask
	one.set(0)
	one.set(64)
	if one.count() != 2 || !one.many() || one.first() != 0 {
		t.Fatalf("slots 0+64: count %d many %v first %d", one.count(), one.many(), one.first())
	}
	if got := one.rest(); got.has(0) || !got.has(64) || got.count() != 1 {
		t.Fatalf("rest of 0+64 is wrong")
	}
	if n := m.not(); n.has(64) || n.has(65) || n.count() != MaxPerm-2 {
		t.Fatalf("not of 64+65: count %d", n.count())
	}
}
