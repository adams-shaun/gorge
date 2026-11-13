package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gorge's per-ADD state.Game.Entered, bounded.
//
// Every zone add this turn is appended here. A Count$ThisTurnEntered_<Dest>
// [_from_<Origin>]_<Valid> head walks the list and matches the object
// wherever it now lives, so a card that entered twice counts twice and a
// card that has since moved on is still listed. zoneTurn (one bit per object
// for its CURRENT zone) cannot express either, which is why this ring exists.
// Stack adds are not recorded, except a cast from a graveyard (the card
// leaves it for the stack: cast_leftgraveyard.go, undone by a reversal);
// kgen refuses a dest Stack head. Draws are recorded at the draw site
// (library->hand).
//
// The ring is fixed size and stops at capacity: the Nth+1 add stalls with
// StallEntered rather than truncating, so a generated table never reads a
// short list as if it were complete.

// maxEntered is the zone entries one turn may record. Only tables with a
// reader carry the array. Synthetic Standard pools reach 47 at 300 pairs; a
// loop can reach more, and stalls.
const maxEntered = 64

// zoneEntry is one add. to/from are kop.Zone values (gorge's ZoneEntry.To
// and .From); flags bit 0 is gorge's WasCreature, the creature status at a
// departure (CR 603.10a) or the card's face at a battlefield entry; bit 1 is the controller a battlefield departure left
// under, gorge's Controller. Owner, PermanentCard and Sacrificed are not read by
// any Count$ThisTurnEntered head and are not ported.
type zoneEntry struct {
	o     Obj
	to    uint8
	from  uint8
	flags uint8
}

const (
	enteredWasCreature uint8 = 1
	enteredLeftCtrl1   uint8 = 2
)

// noteLeftController records on o's battlefield departure, just added, the
// controller it left under (CR 603.10a).
//
//go:noinline
func (s *State) noteLeftController(o Obj, ctrl uint8) {
	if n := s.g.enteredN; n != 0 && !s.g.enteredFull && ctrl&1 != 0 {
		if e := &s.cold.entered[n-1]; e.o == o && e.from == uint8(kop.ZoneBattlefield) {
			e.flags |= enteredLeftCtrl1
		}
	}
}

// matchDeparture is matchCardEntered for a battlefield departure: "you control" reads
// the controller it left under, "you own" its owner.
func (s *State) matchDeparture(k *prep, f kop.FilterID, e *zoneEntry, ctx evalCtx) bool {
	if f == 0 {
		return true
	}
	ob := &s.objs[e.o]
	left := e.leftController()
	for ; f != 0; f = k.t.Filters[f].Or {
		fl := k.t.Filters[f]
		if fl.Flags&kop.FOther != 0 && e.o == ctx.src || fl.Flags&kop.FSelf != 0 && e.o != ctx.src {
			continue
		}
		if fl.Ctrl == kop.CtrlYou && left != ctx.ctrl || fl.Ctrl == kop.CtrlOpp && left == ctx.ctrl {
			continue
		}
		fl.Ctrl = kop.CtrlAny
		if s.matchCardOne(k, &fl, ob.card, ob.owner(), ctx.ctrl, e.o) {
			return true
		}
	}
	return false
}

// leftController is the controller a battlefield departure's entry left under.
func (e *zoneEntry) leftController() uint8 {
	if e.flags&enteredLeftCtrl1 != 0 {
		return 1
	}
	return 0
}

// noteEnter appends one add. The ring never overwrites: once maxEntered
// entries are stored the next add sets enteredFull and stalls (a generated
// table stops at its next step), so the valid entries are always
// entered[0:enteredN] in add order.
//
//go:noinline
func (s *State) noteEnter(o Obj, to, from kop.Zone, wasCreature bool) {
	if !s.g.enteredOn || s.g.enteredFull || s.turn == 0 {
		return
	}
	if s.g.enteredN == maxEntered {
		s.g.enteredFull = true
		s.capFull(StallEntered)
		return
	}
	e := &s.cold.entered[s.g.enteredN]
	e.o, e.to, e.from, e.flags = o, uint8(to), uint8(from), 0
	if to == kop.ZoneBattlefield {
		wasCreature = s.k().t.Cards[s.objs[o].card].Types&kop.TCreature != 0
	}
	if wasCreature {
		e.flags |= enteredWasCreature
	}
	s.g.enteredN++
}

// enteredReferenced reports whether the ring still names o. newToken must
// not recycle the object while a count could read it.
//
//go:noinline
func (s *State) enteredReferenced(o Obj) bool {
	for i := uint8(0); i < s.g.enteredN; i++ {
		if s.cold.entered[i].o == o {
			return true
		}
	}
	return false
}

// thisTurnEntered evaluates Expr ExThisTurnEntered. dest is Expr.N; From is
// -1 when the spec has no from_ origin. The match is gorge's
// matchesEnteredEntry:
//
//   - a battlefield departure is matched as the object where it now lives
//     (CR 400.7), with the spec's Creature-base alternatives rewritten onto
//     Card and gated on the entry's WasCreature bit;
//   - every other entry is matched against the destination zone, and a copy
//     that is neither on the stack nor on the battlefield has ceased to
//     exist (CR 707.10h) and matches nothing there -- a battlefield
//     DESTINATION is not "there": the entry proves the copy was a permanent;
//   - dest=Battlefield matches the live permanent when the object is still
//     there; otherwise it matches the card form, where the Permanent base
//     means a permanent CARD (printed permanent type), not a live permanent.
//     A copy that entered and has since ceased still matches its card form.
//
// Prop sums the face value plus P/T counter deltas (counters exist only on
// the battlefield) or the mana value.
//
//go:noinline
func (s *State) thisTurnEntered(k *prep, x *kop.Expr, ctx evalCtx) int {
	if DebugEnteredLive {
		return s.thisTurnEnteredLive(k, x, ctx)
	}
	dest := kop.Zone(x.N)
	n := 0
	for i := uint8(0); i < s.g.enteredN; i++ {
		e := &s.cold.entered[i]
		if e.to != uint8(dest) {
			continue
		}
		if x.From >= 0 && int8(e.from) != x.From {
			continue
		}
		ob := &s.objs[e.o]
		fromBattlefield := e.from == uint8(kop.ZoneBattlefield)
		ok := false
		if dest == kop.ZoneBattlefield {
			if x.F2 == 0 {
				ok = s.matchBattlefieldEntry(k, x.F, x.F4, e, ctx)
			} else {
				// the Creature-base alternatives read what entered, not what it is now
				ok = (e.flags&enteredWasCreature != 0 || s.faceDownAt(k, e.o)) && s.matchBattlefieldEntry(k, x.F2, x.F2, e, ctx)
				if !ok && x.F3 != 0 {
					ok = s.matchBattlefieldEntry(k, x.F3, x.F3, e, ctx)
				}
			}
		} else if x.F2 != 0 && fromBattlefield {
			// gorge's CR 603.10a split: the Creature-base alternatives
			// become Card alternatives, gated on the entry's departure-time
			// creature status; every other alternative is unchanged.
			if e.flags&enteredWasCreature != 0 {
				ok = s.matchDeparture(k, x.F2, e, ctx)
			}
			if !ok && x.F3 != 0 {
				ok = s.matchDeparture(k, x.F3, e, ctx)
			}
		} else if fromBattlefield {
			ok = s.matchDeparture(k, x.F, e, ctx)
		} else if !s.ceasedCopy(ob) {
			ok = s.matchCardEntered(k, x.F, ob.card, ob.owner(), ctx.ctrl, e.o, ctx.src)
		}
		if !ok {
			continue
		}
		if x.Prop == kop.PropNone {
			n++
		} else {
			n += s.enteredValue(k, e, x.Prop)
		}
	}
	return n
}

// matchBattlefieldEntry matches a dest=Battlefield entry: the live permanent while the
// object is still there, else its card form. The entry itself proves the object
// was a permanent on the battlefield, so a copy that has since ceased still
// matches its card form; CR 707.10h's rejection is for an off-battlefield
// destination, applied by thisTurnEntered's other branch.
func (s *State) matchBattlefieldEntry(k *prep, perm, card kop.FilterID, e *zoneEntry, ctx evalCtx) bool {
	ob := &s.objs[e.o]
	if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		return s.matchPermGen(k, perm, ob.slot, ctx.ctrl, ctx.srcSl, layerFull)
	}
	return s.matchCardEntered(k, card, ob.card, ob.owner(), ctx.ctrl, e.o, ctx.src)
}

// DebugEnteredLive is the mutation test's switch: it makes thisTurnEntered
// read zoneTurn (an object whose CURRENT zone is dest) instead of the
// per-add ring. That is the reader the round-t3 review refused: it cannot
// see a double entry and forgets a card that moved on. Test-only and cold.
var DebugEnteredLive bool

// thisTurnEnteredLive is the refused zoneTurn reader, kept only for the
// mutation test: it counts an object when its CURRENT zone is dest and it
// was put there this turn, so a double entry counts once and a card that
// entered and moved on is lost.
//
//go:noinline
func (s *State) thisTurnEnteredLive(k *prep, x *kop.Expr, ctx evalCtx) int {
	dest := kop.Zone(x.N)
	n := 0
	for o := Obj(0); o < Obj(s.nobj); o++ {
		ob := &s.objs[o]
		if ob.zone != dest || !s.g.zoneTurn.has(o) {
			continue
		}
		ok := false
		if dest == kop.ZoneBattlefield {
			if ob.slot != NoSlot {
				ok = s.matchPermGen(k, x.F, ob.slot, ctx.ctrl, ctx.srcSl, layerFull)
			}
		} else if !s.ceasedCopy(ob) {
			ok = s.matchCardEntered(k, x.F, ob.card, ob.owner(), ctx.ctrl, o, ctx.src)
		}
		if ok {
			n++
		}
	}
	return n
}

// matchCardEntered is matchCardGen for a zone-entry ring read. The card
// matcher has no source object, so it cannot apply the filter's Self/Other
// flags; the ring does know the source (the ability or static's object) and
// applies them here, clause by clause, before matchCardOne. Every other flag
// and predicate stays matchCardOne's.
//
//go:noinline
func (s *State) matchCardEntered(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o, src Obj) bool {
	if f == 0 {
		return true
	}
	for f != 0 {
		fl := &k.t.Filters[f]
		if fl.Flags&kop.FOther != 0 && o == src {
			f = fl.Or
			continue
		}
		if fl.Flags&kop.FSelf != 0 && o != src {
			f = fl.Or
			continue
		}
		if s.matchCardOne(k, fl, c, owner, ctrl, o) {
			return true
		}
		f = fl.Or
	}
	return false
}

// ceasedCopy is CR 707.10h: a copy that is neither on the stack (still a
// spell) nor on the battlefield (still a permanent) has ceased to exist.
func (s *State) ceasedCopy(ob *object) bool {
	return ob.of&ofCopy != 0 && ob.zone != kop.ZoneBattlefield && ob.zone != kop.ZoneStack
}

// enteredValue is gorge's objectProperty over one entry's object. A live
// permanent reads its copy face (gorge's Face() is CopyFace), so a Clone
// that entered and is still on the battlefield sums the copied card's
// printed values; once it leaves, the copy face is gone and the card's own
// values are read.
func (s *State) enteredValue(k *prep, e *zoneEntry, prop uint8) int {
	ob := &s.objs[e.o]
	c := ob.card
	live := ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot
	if live {
		c = s.cardAt(ob.slot)
	}
	cd := &k.t.Cards[c]
	switch prop {
	case kop.PropCardPower:
		n := int(cd.Power)
		if live {
			p := &s.perms[ob.slot]
			n += p.ctrLow(int(kop.CtrP1P1)) - p.ctrLow(int(kop.CtrM1M1))
		}
		return n
	case kop.PropCardToughness:
		n := int(cd.Toughness)
		if live {
			p := &s.perms[ob.slot]
			n += p.ctrLow(int(kop.CtrP1P1)) - p.ctrLow(int(kop.CtrM1M1))
		}
		return n
	case kop.PropCardManaCost:
		if live && s.faceDown(k, ob.slot) {
			return 0 // CR 708.2: a face-down permanent has no mana cost
		}
		return int(k.cards[c].total)
	}
	return 0
}
