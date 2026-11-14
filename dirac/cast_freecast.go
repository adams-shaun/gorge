package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// freeQueued: o is an OpCastFree pick still waiting to be cast after the
// resolution (genState.freeQ). gorge casts a discovered card at once, so
// the discover tail must not send it to the hand.
func (s *State) freeQueued(o Obj) bool {
	for _, q := range s.g.freeQ[:s.g.nfq] {
		if q == o {
			return true
		}
	}
	return false
}

// the four api:Play free-cast spellings on OpCastFree, plus the
// view-only top-of-library MayLookAt static (kgen lowers that one to no
// instruction, so nothing here reads it).
//
// OpCastFree's operands:
//   - C low 3 bits: the population zone (Exile / Hand / Graveyard / Library /
//     Command: the resolving controller's own commanders).
//   - C bit 3: Amount$ 1 (close the ask after one pick).
//   - C bit 4: ForgetPlayed$ True (drop the card from s.g.rem once its cast
//     commits; a refused or reversed cast never commits and keeps it).
//   - C bit 5: ReplaceGraveyard$ Exile (exile when it leaves the stack).
//   - C bit 6: CopyCard$ True; bit 7: AllowRepeats$ True with Amount$ A (cast_freerepeat.go).
//   - C bit 8: the copy pays its mana cost; bit 9: a literal Amount$ A (the ask closes after A picks).
//   - Obj: RefRemembered (P1), a target slot (P2: the targeted card; a
//     player target: that player's hand), or RefNone with F (P3/P4 filter
//     population over the zone).
//
// The zone, ReplaceGraveyard and ForgetPlayed bits ride to the freeQ drain
// in s.g.freeP: the caster in the low freeSeatBits, the zone in the three
// bits above, then freeGY (ReplaceGraveyard), freeCopy (CopyCard), freePaid
// and freeForget (ForgetPlayed). At two seats that is bit 0, bits 1-3 and
// bits 4-7 of a byte.
const (
	freeSeatBits  = MaxSeats / 2 // one bit at two seats, two at four
	freeSeatMask  = 1<<freeSeatBits - 1
	freeZoneShift = freeSeatBits
	freeGY        = 1 << (freeZoneShift + 3)
	freeCopy      = freeGY << 1
	freePaid      = freeGY << 2
	freeForget    = freeGY << 3
)

// freeSeatBits holds every seat.
var _ = [1<<freeSeatBits - MaxSeats]struct{}{}

// sfFreeGY is stackEntry.flags bit 4, meaning "exile instead of the
// graveyard" on a free cast. Bit 4 also means sfFbGranted while a cast is built
// (cleared at finishBuild). A free cast's bit is set after finishBuild, in
// cast.go, and read here by finishResolve and counterAt, so the two never
// overlap.
const sfFreeGY uint8 = 1 << 4

// exilesOnLeave reports whether a spell entry leaving the stack for the
// graveyard is exiled instead: the flashback family, or a free cast with
// ReplaceGraveyard$ Exile (gorge state.ExilesInsteadOfGraveyard). The one
// predicate every stack exit reads (finishResolve, the held-spell exit,
// counterAt), so a new exit cannot miss the rider.
func (e *stackEntry) exilesOnLeave() bool {
	return e.flags&sfFlashback != 0 || e.flags&sfFree != 0 && e.flags&sfFreeGY != 0 && !DebugM11NoReplaceGY ||
		e.kind == seSpell && e.remVer&spellExileLeave != 0 // an Effect's exile-instead mark (repl_stackexile.go)
}

// Debug switches for the mutation test (the counters live on
// the gorge side of the lockstep, not in the kernel).
var (
	// DebugM11IgnoreOne makes Amount$ 1 offer every candidate (mutation).
	DebugM11IgnoreOne bool
	// DebugM11NoReplaceGY keeps a ReplaceGraveyard spell's normal
	// graveyard destination (mutation).
	DebugM11NoReplaceGY bool
	// DebugFreeFilterIgnored skips the free-cast population's Valid$/ValidSA$
	// filter, so a cmc bound (Captain Benjamin Sisko's Officer count) cuts
	// nothing (mutation).
	DebugFreeFilterIgnored bool
)

// castFreeFillGen fills s.g.free for the target and filter populations. The
// remembered population (P1) reuses the same gates; the caller keeps the
// Etali path inline and reaches here only for the new operands.
//
//go:noinline
func (s *State) castFreeFillGen(k *prep, in *kop.Instr, e *stackEntry) {
	zone := kop.Zone(in.C & 7)
	add := func(o Obj) bool {
		ob := &s.objs[o]
		if ob.zone != zone || k.cards[ob.card].land {
			return true
		}
		if in.F != 0 && !DebugFreeFilterIgnored && !s.matchCardGen(k, in.F, ob.card, ob.owner(), e.ctrl, o) {
			return true
		}
		// CR 601.2c: a spell whose mandatory targets cannot all be chosen is
		// never offered (gorge effPlay filters through castTargetsHost)
		if !s.spellTargetsFeasibleFrom(k, ob.card, e.ctrl, o) {
			return true
		}
		if k.gen.anyCantCast && s.castForbiddenFree(k, e.ctrl, ob.card, o) {
			return true
		}
		if s.g.nfree >= uint8(len(s.g.free)) {
			s.capFull(StallFree)
			return false
		}
		s.g.free[s.g.nfree] = o
		s.g.nfree++
		return true
	}
	switch {
	case in.Obj == kop.RefRemembered:
		for i := 0; i < int(s.g.nrem); i++ {
			if !add(s.g.rem[i]) {
				return
			}
		}
	case in.Obj == kop.RefSelf:
		add(e.obj) // the source card in exile (an exile-granted ability's Play; cast_exilegrant.go)
	case tgtSlotOf(in.Obj) >= 0:
		if slot := tgtSlotOf(in.Obj); slot < int(e.nt) {
			if o := Obj(e.tgt[slot]); isObjTarget(TCode(o)) {
				add(o)
			} else if isPlayerTarget(e.tgt[slot]) {
				s.castFreeFillHand(CodeSeat(e.tgt[slot]), zone, add) // cast_freetgtplayer.go
			}
		}
	case zone == kop.ZoneHand:
		pl := &s.pl[e.ctrl]
		for i := 0; i < int(pl.handN); i++ {
			if !add(pl.hand[i]) {
				return
			}
		}
	case zone == kop.ZoneCommand:
		// P5: the resolving controller's own commanders, in its command-zone
		// order (gorge effPlay walks that seat's command zone; Geode Golem)
		first, second := s.cmdZoneOrder(e.ctrl)
		for _, o := range [2]Obj{first, second} {
			if o != NoObj && !add(o) {
				return
			}
		}
	case zone == kop.ZoneGraveyard:
		// gorge AliveFrom(0): seat 0's graveyard then seat 1's, each in
		// zone order
		for p := 0; p < int(s.nSeats()); p++ {
			if s.out(uint8(p)) {
				continue
			}
			pl := &s.pl[p]
			for i := 0; i < int(pl.graveN); i++ {
				if !add(pl.grave[i]) {
					return
				}
			}
		}
	}
	if in.C&128 != 0 {
		s.castFreeRepeat(k, in) // cast_freerepeat.go: AllowRepeats$ with Amount$ X
	}
}

// forgetRemGen drops o from the resolution's remembered set, order-preserving
// (gorge forgetRememberedOne; ForgetPlayed$ True drops the begun card on the
// PlayDone pass, i.e. when the answer is applied, before the Sub chain).
func (s *State) forgetRemGen(o Obj) {
	for i := 0; i < int(s.g.nrem); i++ {
		if s.g.rem[i] == o {
			copy(s.g.rem[i:], s.g.rem[i+1:s.g.nrem])
			s.g.nrem--
			return
		}
	}
}
