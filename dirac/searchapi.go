package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Accessors and world operations for a search (no allocation).

// notObj is the identity accessors' answer for a byte past MaxObj, which a
// search client reads an Action's Target() as when the target is not an
// object: a stack-ability target (StackTarget0 + its stack index,
// tgt_stackability.go) names its stack entry (ok, e non-nil); NoObj, a
// player code or an index past the stack names nothing (e nil). Without
// it a client's ObjCard(Obj(tg)) read s.objs past its end (index out of
// range [224] with length 192, a TRK search game's panic).
func (s *State) notObj(o Obj) (e *stackEntry, ok bool) {
	if int(o) < MaxObj {
		return nil, false
	}
	if i, is := StackTargetIndex(uint8(o)); is && i < int(s.sp) {
		return &s.stack[i], true
	}
	return nil, true
}

// ObjCard is object o's card (its copy-of card on the battlefield); a
// stack-ability target byte's is its source card.
func (s *State) ObjCard(o Obj) CardID {
	if e, no := s.notObj(o); no {
		if e == nil {
			return 0
		}
		return e.card
	}
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield {
		return s.cardAt(ob.slot)
	}
	return ob.card
}

// ObjOwner, ObjZone and ObjToken describe object o.
func (s *State) ObjOwner(o Obj) uint8 {
	if e, no := s.notObj(o); no {
		if e == nil {
			return 0
		}
		return e.ctrl // an ability has no owner: its controller
	}
	return s.objs[o].owner()
}

func (s *State) ObjZone(o Obj) kop.Zone {
	if e, no := s.notObj(o); no {
		if e == nil {
			return kop.ZoneNone
		}
		return kop.ZoneStack
	}
	return s.objs[o].zone
}

func (s *State) ObjToken(o Obj) bool {
	if _, no := s.notObj(o); no {
		return false
	}
	return s.objs[o].of&ofToken != 0
}

func (s *State) NumObjs() int            { return int(s.nobj) }
func (s *State) ActivePlayer() uint8     { return s.active }
func (s *State) StepNow() kop.Step       { return s.step }
func (s *State) HandSize(p uint8) int    { return int(s.pl[p].handN) }
func (s *State) LibrarySize(p uint8) int { return s.pl[p].libLen() }

// Controller is the controller of a battlefield object (its owner
// elsewhere).
func (s *State) Controller(o Obj) uint8 {
	if e, no := s.notObj(o); no {
		if e == nil {
			return 0
		}
		return e.ctrl
	}
	return s.ctrlOfObj(o)
}

// PowerToughness is a battlefield creature's derived P/T.
func (s *State) PowerToughness(o Obj) (int, int) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return 0, 0
	}
	return s.pt(s.k(), ob.slot)
}

// IsCreature reports whether battlefield object o is a creature now.
func (s *State) IsCreature(o Obj) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneBattlefield && s.isCreature(s.k(), ob.slot)
}

// Hidden reports whether object o's identity is hidden from seat: the
// other player's hand (unless HandRevealed) and library, and every
// library (order).
func (s *State) Hidden(o Obj, seat uint8) bool {
	ob := &s.objs[o]
	switch ob.zone {
	case kop.ZoneLibrary:
		return true
	case kop.ZoneHand:
		return ob.owner() != seat && !s.HandRevealed(ob.owner(), seat)
	}
	return false
}

// HandRevealed reports whether player p's hand is revealed to seat now:
// seat is choosing a card from it mid-resolution (Pilfer's "reveals their
// hand; you choose a nonland card"). A search world keeps such a hand as
// it is: its cards are the choice's options.
func (s *State) HandRevealed(p, seat uint8) bool {
	return p != seat && s.ctl == ctlResolve && s.res.ask == raDiscard && s.res.chosen == p && s.res.askP == seat
}

// DealLibrary deals pool (len = p's library size) to player p's library in
// order (a world whose hand of p is revealed, HandRevealed).
func (s *State) DealLibrary(p uint8, pool []CardID) {
	pl := &s.pl[p]
	for i := 0; i < len(pool) && int(pl.libLo)+i < int(pl.libHi); i++ {
		s.dealCard(pl.lib[int(pl.libLo)+i], pool[i])
	}
}

// Rand is a value PCG stream with gorge's IntN reduction (rand.Rand's),
// for search worlds.
type Rand struct{ r rng }

// NewRand seeds a stream PCG(a, b) (rand.NewPCG(a, b)).
func NewRand(a, b uint64) Rand {
	var x Rand
	x.r.pcg.Seed(a, b)
	return x
}

// IntN is rand.Rand.IntN.
func (x *Rand) IntN(n int) int { return x.r.intN(n) }

// Uint64 is the next raw value.
func (x *Rand) Uint64() uint64 { return x.r.pcg.Uint64() }

// ShuffleCards is rand.Rand.Shuffle over l.
func (x *Rand) ShuffleCards(l []CardID) {
	for i := len(l) - 1; i > 0; i-- {
		j := x.r.intN(i + 1)
		l[i], l[j] = l[j], l[i]
	}
}

// RedealOwnLibrary re-deals the order of seat's own library (its hand is
// known): the library's cards are shuffled among its positions (gorge
// redealPlayer with handFree 0).
func (s *State) RedealOwnLibrary(seat uint8, r *Rand) {
	pl := &s.pl[seat]
	var pool [MaxDeck + 8]CardID
	n := 0
	for i := pl.libLo; i < pl.libHi; i++ {
		pool[n] = s.objs[pl.lib[i]].card
		n++
	}
	r.ShuffleCards(pool[:n])
	for i := 0; i < n; i++ {
		s.dealCard(pl.lib[int(pl.libLo)+i], pool[i])
	}
}

// HiddenCards writes the cards player p's hand and library hold now into
// out (hand first) and returns how many (the open-list pool).
func (s *State) HiddenCards(p uint8, out []CardID) int {
	pl := &s.pl[p]
	n := 0
	for i := 0; i < int(pl.handN); i++ {
		out[n] = s.objs[pl.hand[i]].card
		n++
	}
	for i := pl.libLo; i < pl.libHi; i++ {
		out[n] = s.objs[pl.lib[i]].card
		n++
	}
	return n
}

// SeenCards writes player p's cards seat can see (not tokens; battlefield,
// graveyard, exile, stack) into out and returns how many (closed-list
// subtraction).
func (s *State) SeenCards(p, seat uint8, out []CardID) int {
	n := 0
	for o := 0; o < int(s.nobj); o++ {
		ob := &s.objs[o]
		if ob.owner() != p || ob.of&ofToken != 0 || ob.zone == kop.ZoneNone || s.Hidden(Obj(o), seat) {
			continue
		}
		if n < len(out) {
			out[n] = ob.card
			n++
		}
	}
	return n
}

// DealHidden deals pool (len = p's hand + library size) to player p's
// hidden objects as gorge redealPlayer does: shuffle, the first hand-size
// cards to the hand, the rest shuffled again into the library.
func (s *State) DealHidden(p uint8, r *Rand, pool []CardID) {
	pl := &s.pl[p]
	r.ShuffleCards(pool)
	h := int(pl.handN)
	for i := 0; i < h && i < len(pool); i++ {
		s.dealCard(pl.hand[i], pool[i])
	}
	rest := pool[h:]
	r.ShuffleCards(rest)
	for i := 0; i < len(rest) && int(pl.libLo)+i < int(pl.libHi); i++ {
		s.dealCard(pl.lib[int(pl.libLo)+i], rest[i])
	}
}

// dealCard gives hidden object o card c (marked changed only if it was
// another card).
func (s *State) dealCard(o Obj, c CardID) {
	if s.objs[o].card != c {
		s.objs[o].card = c
		s.touch(o)
	}
}

// ReseedChance re-seeds the game's RNG (a world's own future chance).
func (s *State) ReseedChance(a, b uint64) { s.rng.pcg.Seed(a, b) }

// Table is the game's card table.
func (s *State) Table() *Table { return s.k().t }

// Building describes the spell or ability being put on the stack (its
// card, ability index and controller); ok is false outside a build.
func (s *State) Building() (card CardID, ab uint8, ctrl uint8, ok bool) {
	if s.ctl != ctlBuild {
		return 0, 0, 0, false
	}
	return s.bld().e.card, s.bld().e.ab, s.bld().e.ctrl, true
}

// Life is a player's life total (alias of the int form for search code).
func (s *State) LifeOf(p uint8) int { return int(s.pl[p].life) }

// Winner is the winner of a finished game (-1 draw or not over).
func (s *State) Winner() int8 { return s.win }

// Over reports whether the game has ended.
func (s *State) Over() bool { return s.over }

// SeqNo counts the decisions answered so far, elided ones included: a
// per-decision sequence number (gorge's Decision.Seq role: the search's
// decision seed, the network's same-decision memo, a visit record's
// sequence).
func (s *State) SeqNo() uint32 { return s.Decisions }

// Tapped reports whether battlefield object o is tapped.
func (s *State) Tapped(o Obj) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneBattlefield && s.perms[ob.slot].flags&pfTapped != 0
}

// ControlsMatching reports whether p controls a permanent matching filter f,
// read as a return cost's candidates are, with no source on the battlefield.
func (s *State) ControlsMatching(p uint8, f kop.FilterID) bool {
	k := s.k()
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		if s.matchPerm(k, f, m.first(), p, NoSlot) {
			return true
		}
	}
	return false
}

// HasKeyword reports whether battlefield object o has keyword kw now.
func (s *State) HasKeyword(o Obj, kw kop.KwMask) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneBattlefield && s.kw(s.k(), ob.slot)&kw != 0
}

// InHandEver reports whether card object o was in its owner's hand during
// the game (the kept opening hand and every card put into the hand).
func (s *State) InHandEver(o Obj) bool { return s.objs[o].of&ofInHand != 0 }

// Mulligans is the number of mulligans player p took.
func (s *State) Mulligans(p uint8) int { return int(s.pl[p].mulls) }

// DeckSize is the number of card objects seat p's deck started with.
func (s *State) OwnerOf(o Obj) uint8 { return s.objs[o].owner() }

// StartingPlayer is the seat that took the first turn.
func (s *State) StartingPlayer() uint8 { return s.startPl }

// Register prepares tbl for play (the per-table precomputation New does on
// first use); a hot loop registers its tables up front.
func Register(tbl *Table) { registerTable(tbl) }

// Accessors for dirac/search/hbot (read-only, no allocation).

// BuildTargetSlot is the target slot (the index in the ability's
// TargetSpecs) the pending target decision asks; ok is false outside a
// build's target stage.
func (s *State) BuildTargetSlot() (slot int, ok bool) {
	if s.ctl != ctlBuild || s.bld().stage != bsTargets {
		return 0, false
	}
	return int(s.bld().ti), true
}

// PoolOf is player p's floating mana W U B R G C (restricted mana
// included), and its restricted part per colour (every batch's).
func (s *State) PoolOf(p uint8) (pool, restr [kop.NumManaKinds]uint8) {
	s.rbUnits(p, s.rbAll(p), &restr)
	return s.poolProj(p), restr
}

// StackLen is the number of stack entries; StackEntry is entry i (bottom
// 0): its kind (0 spell, 1 activated, 2 triggered), source object, card
// and controller.
func (s *State) StackLen() int { return int(s.sp) }

func (s *State) StackEntry(i int) (kind uint8, src Obj, card CardID, ctrl uint8) {
	e := &s.stack[i]
	return e.kind, e.obj, e.card, e.ctrl
}

// Damage is battlefield object o's marked damage.
func (s *State) Damage(o Obj) int {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return 0
	}
	return int(s.perms[ob.slot].damage)
}

// StackAbility is stack entry i's ability index in its card's CardDef (255
// none or delayed).
func (s *State) StackAbility(i int) uint8 { return s.stack[i].ab }

// Counters is battlefield object o's count of counter kind k.
func (s *State) Counters(o Obj, k kop.CounterKind) int {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return 0
	}
	var po ProjObj
	s.projObj(s.k(), o, &po)
	if k >= kop.CtrCharge && k < kop.NumCounterKinds3 {
		return int(po.CountersX[k-kop.CtrCharge])
	}
	if k >= kop.NumCounterKinds2 {
		return 0
	}
	return int(po.Counters[k])
}

// PicksLeft is how many more picks the pending multi-pick choice owes
// (gorge's Max for the answer still to come): a cost's picks while a
// spell or ability is built, a resolution discard's or sacrifice's, a
// generated ask's, cleanup's discards to hand size; 1 when it cannot say.
func (s *State) PicksLeft() int {
	n := 1
	switch s.ctl {
	case ctlCleanupDiscard:
		n = int(s.discardLeft)
	case ctlBuild:
		n = int(s.bld().pickLeft)
	case ctlResolve:
		switch s.res.ask {
		case raDiscard:
			n = int(s.res.left) - int(s.res.peekI)
		case raSacrifice:
			n = int(s.res.left)
		default:
			n = int(s.g.pickLeft)
		}
	}
	if n < 1 {
		n = 1
	}
	return n
}

// BuildingCast is the spell or ability being built's kicked flag and its
// announced X (ok false outside a build).
func (s *State) BuildingCast() (kicked bool, x int, ok bool) {
	if s.ctl != ctlBuild {
		return false, 0, false
	}
	return s.bld().e.flags&sfKicked != 0, int(s.bld().e.x), true
}
