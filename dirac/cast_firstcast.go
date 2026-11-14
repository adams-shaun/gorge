package dirac

import "sync/atomic"

// ExFirstCast: "this spell is the first spell you've cast this game" (First
// Contact). Forge writes Count$YouCastThisGame into an SVar from a static
// SpellCast trigger on the spell itself, at cast, because a response could add
// casts before the spell resolves; gorge's Host.SpellsCastThisGame counts the
// controller's un-reversed PutOnStack events (CR 601.2i: a spell is cast once
// it is paid for; a copy is never cast). The kernel keeps no counter, since
// only "equals 1" is read: genState.cast1 holds a bit per seat, "has cast a
// spell this game", and the spell entry's remVer carries spellFirstCast when
// its cast was that seat's first. The stamp is taken when the cast completes
// (before the SpellCast triggers queue), so a spell cast in response, which
// resolves first, does not take it away. A stack copy keeps the stamp: gorge's
// copy resolves against the original object's stored SVar (Forge copies the
// card's SVars with the spell), so a copy of a spell stamped first gains too,
// although a copy is never cast (CR 707.10). Both exist only for a table with
// an ExFirstCast (genPrep.anyFirstCast).

// spellFirstCast is the spell entry's remVer bit 2 (spellGainedDeathtouch and
// spellGainedLifelink hold bits 0 and 1; a spell has no remembered object).
const spellFirstCast objWord = 1 << 2

// DebugFirstCastLate is the mutation switch: the flag is read at resolution
// instead of stored at cast. A spell the seat casts later takes the flag from
// every earlier spell of its own on the stack, as a count read at resolution
// would (two casts by then), so a lockstep that responds to First Contact
// with another spell diverges.
var DebugFirstCastLate bool

// FirstCastStats counts the lockstep test's coverage: spells stamped as a
// seat's first cast, spells cast after that seat's first, and resolutions
// that read the stamp as true.
var FirstCastStats struct{ Stamped, Later, ReadTrue atomic.Int64 }

// noteFirstCast records the cast that just completed: the entry on top of the
// stack is the spell. The seat's first cast stamps it.
func (s *State) noteFirstCast() {
	if s.sp == 0 {
		return
	}
	e := &s.stack[s.sp-1]
	if e.kind != seSpell {
		return
	}
	if DebugFirstCastLate {
		for i := uint8(0); i < s.sp-1; i++ {
			if o := &s.stack[i]; o.kind == seSpell && o.ctrl == e.ctrl {
				o.remVer &^= spellFirstCast
			}
		}
	}
	bit := uint8(1) << e.ctrl
	if s.g.cast1&bit == 0 {
		s.g.cast1 |= bit
		e.remVer |= spellFirstCast
		FirstCastStats.Stamped.Add(1)
	} else {
		FirstCastStats.Later.Add(1)
	}
}

// firstCast reports whether e, the resolving entry (a copy included), is a spell stamped as its
// controller's first cast this game. e == nil or a non-spell entry reads false.
func (s *State) firstCast(e *stackEntry) bool {
	if e != nil && e.kind == seSpell && e.remVer&spellFirstCast != 0 {
		FirstCastStats.ReadTrue.Add(1)
		return true
	}
	return false
}
