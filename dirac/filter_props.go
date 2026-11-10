package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugWord7Flip is TestFilterWordsZoneAndPowerMutation's switch: every PredWord7 answer is negated.
var DebugWord7Flip bool

// wordPerm is PredWord7 on a battlefield permanent (gorge
// effects/filter.go hasNoName, greatestPower, attackingAlone and filter_word.go
// inZone<Zone>).
//
//go:noinline
func (s *State) wordPerm(k *prep, fl *kop.Filter, sl Slot) bool {
	if fl.PredN&0xff >= kop.W7FaceUp {
		return s.wordPerm2(k, fl, sl, s.ctxCur().ctrl, NoSlot) // the walk path (matchPermOne passes its own ctrl and src)
	}
	ans := false
	switch fl.PredN & 0xff {
	case kop.W7NoName:
		// CR 708.2a face down, or CR 709.5 a Room with no door unlocked
		c := s.perms[sl].card
		ans = s.faceDown(k, sl) || (roomDef(k, c) && roomMask(k, c) == 0)
	case kop.W7GreatestPower:
		// a battlefield creature no OTHER battlefield creature out-powers; ties match
		if s.isCreature(k, sl) {
			ans = true
			pw := s.power(k, sl)
			for m := s.occ; m.any(); m = m.rest() {
				o2 := m.first()
				if o2 != sl && s.isCreature(k, o2) && s.power(k, o2) > pw {
					ans = false
					break
				}
			}
		}
	case kop.W7AttackingAlone:
		// attacking, and exactly one battlefield permanent is attacking
		if s.perms[sl].flags&pfAttacking != 0 {
			n := 0
			for m := s.occ; m.any(); m = m.rest() {
				if s.perms[m.first()].flags&pfAttacking != 0 {
					n++
				}
			}
			ans = n == 1
		}
	case kop.W7InZone:
		ans = kop.Zone(fl.PredN>>8) == kop.ZoneBattlefield
	}
	return ans != DebugWord7Flip
}

// wordCard is PredWord7 on a card off the battlefield (o may be NoObj).
//
//go:noinline
func (s *State) wordCard(k *prep, fl *kop.Filter, c CardID, o Obj, fd bool, ctrl uint8) bool {
	if fl.PredN&0xff >= kop.W7FaceUp {
		return s.wordCard2(k, fl, c, o, fd, ctrl) // filter_props.go
	}
	ans := false
	switch fl.PredN & 0xff {
	case kop.W7NoName:
		ans = fd // CR 708.2a: a face-down spell; no other nameless card off the battlefield
	case kop.W7InZone:
		ans = o != NoObj && s.objs[o].zone == kop.Zone(fl.PredN>>8)
	}
	return ans != DebugWord7Flip
}

// PredWord7 sub-kinds 5-10 (kop.W7FaceUp ...), cold. The
// battlefield matcher reaches wordPerm2 from matchPermOne (it has ctrl and
// src) behind prep.w8; wordPerm and wordCard delegate here for the walk
// and card paths.

// tableWord8 reports whether some filter is a PredWord7 sub-kind >= W7FaceUp.
func tableWord8(t *kop.Table) bool {
	for i := range t.Filters {
		if f := &t.Filters[i]; f.Pred == kop.PredWord7 && f.PredN&0xff >= kop.W7FaceUp {
			return true
		}
	}
	return false
}

// wordPerm2 is the sub-kinds on a battlefield permanent; ctrl is the
// evaluating controller (gorge sc.You), src the source slot (NoSlot for a spell).
//
//go:noinline
func (s *State) wordPerm2(k *prep, fl *kop.Filter, sl Slot, ctrl uint8, src Slot) bool {
	if fl.PredN&0xff == kop.W7NoNameTwinGrave {
		nm := s.nameCardAt(sl)
		return (nm == NoCard || !s.nameTwinPerm(nm, sl, s.perms[sl].ctrl) && !s.nameInGrave(nm, ctrl)) != DebugWord7Flip
	}
	if fl.PredN&0xff == kop.W7AttachedTo {
		return s.attachedToWord(k, fl.PredN>>8, sl, ctrl, src)
	}
	if fl.PredN&0xff == kop.W7SharesCrTrigObj {
		return s.sharesCrTrigObj(k, s.perms[sl].obj)
	}
	if fl.PredN&0xff == kop.W7Imprinted {
		return s.impHas(s.perms[sl].obj) != DebugWord7Flip
	}
	if fl.PredN&0xff >= kop.W7WithWarp {
		return s.wordPerm3(k, fl, sl, src) // filter_props.go
	}
	ans := false
	switch fl.PredN & 0xff {
	case kop.W7FaceUp:
		// CR 708.2: the negation of face down (gorge wordFaceUp)
		ans = !s.faceDown(k, sl) || s.blankCopy(sl)
	case kop.W7NotChosenType:
		// gorge IsNotChosenType: fails closed when nothing was chosen
		if ch, ok := s.chosenTypeSub(src); ok {
			ans = !s.hasSubGen(k, sl, ch) && !(outsideSub(k, ch) && s.outsideChosenHas(k, sl, src))
		}
	case kop.W7AttackingOpp:
		// attacking a PLAYER who is an opponent of ctrl; a planeswalker defender matches nothing
		if MaxSeats == 2 {
			ans = s.perms[sl].flags&pfAttacking != 0 && s.perms[sl].atk == PlayerCode(NextSeat(ctrl))
		} else {
			t := s.perms[sl].atk
			ans = s.perms[sl].flags&pfAttacking != 0 && isPlayerTarget(t) && IsOpponent(ctrl, CodeSeat(t))
		}
	case kop.W7EnchantedByYours:
		// some Aura attached to the candidate is controlled by ctrl (CR 303.4)
		for m := s.occ; m.any(); m = m.rest() {
			q := m.first()
			if s.perms[q].attached == sl && s.perms[q].ctrl == ctrl && s.typesGen(k, q)&kop.TAura != 0 {
				ans = true
				break
			}
		}
	case kop.W7NoNameTwin:
		// no OTHER permanent its controller controls shares its name (CR 201.2)
		ans = true
		if nm := s.nameCardAt(sl); nm != NoCard {
			for m := s.occ; m.any(); m = m.rest() {
				q := m.first()
				if q != sl && s.perms[q].ctrl == s.perms[sl].ctrl && s.nameCardAt(q) == nm {
					ans = false
					break
				}
			}
		}
	case kop.W7SharesNameLegend:
		// some OTHER legendary battlefield permanent shares its name
		if nm := s.nameCardAt(sl); nm != NoCard {
			for m := s.occ; m.any(); m = m.rest() {
				q := m.first()
				if q != sl && s.nameCardAt(q) == nm && s.typesGen(k, q)&kop.TLegendary != 0 {
					ans = true
					break
				}
			}
		}
	}
	return ans != DebugWord7Flip
}

// nameTwinPerm reports whether a battlefield permanent other than skip that
// player p controls has the name of card nm (gorge doesNotShareNameWith
// OtherYourBattlefield; a phased-out permanent is off occ).
func (s *State) nameTwinPerm(nm CardID, skip Slot, p uint8) bool {
	for m := s.occ; m.any(); m = m.rest() {
		if q := m.first(); q != skip && s.perms[q].ctrl == p && s.nameCardAt(q) == nm {
			return true
		}
	}
	return false
}

// nameInGrave reports whether a card in player p's graveyard has the name of
// card nm (gorge doesNotShareNameWith YourGraveyard).
func (s *State) nameInGrave(nm CardID, p uint8) bool {
	pl := &s.pl[p]
	for i := 0; i < int(pl.graveN); i++ {
		if s.objs[pl.grave[i]].card == nm {
			return true
		}
	}
	return false
}

// wordCard2 is the sub-kinds on a card off the battlefield (o may be NoObj);
// c is the card, ctrl the evaluating controller.
//
//go:noinline
func (s *State) wordCard2(k *prep, fl *kop.Filter, c CardID, o Obj, fd bool, ctrl uint8) bool {
	if n := fl.PredN & 0xff; n == kop.W7BottomLibrary || n == kop.W7NotBottomLibrary {
		return s.bottomLibrary(o) == (n == kop.W7BottomLibrary)
	}
	if fl.PredN&0xff == kop.W7SharesCrTrigObj {
		return o != NoObj && s.sharesCrTrigObj(k, o)
	}
	if fl.PredN&0xff == kop.W7Imprinted {
		return o != NoObj && s.impHas(o) != DebugWord7Flip
	}
	if fl.PredN&0xff == kop.W7EnteredFromBF {
		return o != NoObj && s.enteredFromBattlefield(o) != DebugWord7Flip
	}
	if fl.PredN&0xff == kop.W7NoNameTwinGrave {
		// off the battlefield the card has no other self to skip; in the
		// evaluating controller's graveyard it shares a name with itself
		return (fd || !s.nameTwinPerm(c, NoSlot, ctrl) && !s.nameInGrave(c, ctrl)) != DebugWord7Flip
	}
	if fl.PredN&0xff >= kop.W7WithWarp {
		return s.wordCard3(k, fl, c, fd) // filter_props.go
	}
	ans := false
	switch fl.PredN & 0xff {
	case kop.W7FaceUp:
		ans = !fd // gorge wordFaceUp reads the object in any zone
	case kop.W7NotChosenType:
		if ch, ok := s.chosenTypeSub(NoSlot); ok {
			cd := &k.t.Cards[c]
			has := int(ch) < len(k.t.Subtypes) && cd.Subtypes.Has(int(ch))
			if outsideSub(k, ch) {
				has = cd.AllCreatureTypes
			}
			if s.res.bad&resChosenCard != 0 {
				has = s.chosenCardTypeHas(cd.Types)
			}
			ans = !has
		}
	case kop.W7NoNameTwin:
		ans = true // no permanent ctrl controls shares the card's name
		if !fd {
			for m := s.occ; m.any(); m = m.rest() {
				q := m.first()
				if s.perms[q].ctrl == ctrl && s.nameCardAt(q) == c {
					ans = false
					break
				}
			}
		}
	case kop.W7SharesNameLegend:
		if !fd {
			for m := s.occ; m.any(); m = m.rest() {
				q := m.first()
				if s.nameCardAt(q) == c && s.typesGen(k, q)&kop.TLegendary != 0 {
					ans = true
					break
				}
			}
		}
	}
	// W7AttackingOpp, W7EnchantedByYours: nothing off the battlefield attacks or wears an Aura
	return ans != DebugWord7Flip
}

// PredWord7 sub-kinds 11-13 and 22 (kop.W7WithWarp, kop.W7WithDisguise,
// kop.W7GreatestPowerTgt, kop.W7CrewedBySrc), the Mayhem code of PredWasCastSpell and the
// ManaSpent-EQ0 bit of PredWasCastEnter, cold. wordPerm2 and wordCard2
// delegate every sub-kind >= 11 here. No State byte: the crew record reuses
// the source-mark slots (filter_combatwords.go).

// DebugWord9Flip is TestFilterWordsCrewedAndCastMutation's switch: the PredWord7 answers of
// sub-kinds 11-13, the Mayhem read of wasCastSpell and the ManaSpent-EQ0 read
// of wasCastRead are negated.
var DebugWord9Flip bool

// wasCastSpentZero is PredWasCastEnter's PredN bit for CastSa
// Spell.ManaSpent EQ0 (kgen wasCastSpentZero).
const wasCastSpentZero = 8

// wasCastWebSling is PredWasCastEnter's PredN bit for webSlinged: the
// resolving spell was cast for its web-slinging cost (kgen wasCastWebSling).
const wasCastWebSling = 16

// wordPerm3 is the sub-kinds on a battlefield permanent; src is the source
// slot (NoSlot for a spell or a walk).
//
//go:noinline
func (s *State) wordPerm3(k *prep, fl *kop.Filter, sl Slot, src Slot) bool {
	ans := false
	switch fl.PredN & 0xff {
	case kop.W7WithWarp:
		// gorge withWarp on a battlefield object reads the derived keyword
		// list: printed Warp, none while face down (CR 708.2) or after a
		// lose-all-abilities effect (the kw gate, chars.go)
		c := s.cardAt(sl)
		ans = c != kop.NoCard && k.t.Cards[c].HasWarp && !s.faceDown(k, sl) &&
			s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0
	case kop.W7WithDisguise, kop.W7FaceUpDisguise:
		// gorge withDisguise on a battlefield object: the printed keyword,
		// none while face down or after a lose-all-abilities effect
		c := s.cardAt(sl)
		ans = c != kop.NoCard && k.t.Cards[c].MorphFamily == kop.MorphDisguise && !s.faceDown(k, sl) &&
			s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0
	case kop.W7WithFlashback:
		// gorge withFlashback on a battlefield object: the printed keyword,
		// none while face down or after a lose-all-abilities effect
		c := s.cardAt(sl)
		ans = c != kop.NoCard && k.t.Cards[c].Keywords&kop.KwFlashback != 0 && !s.faceDown(k, sl) &&
			s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0
	case kop.W7GreatestPowerTgt:
		// gorge greatestPowerControlledByTargeted: a battlefield creature a
		// player target of the current entry controls that no OTHER such
		// creature out-powers; ties match; no player target matches nothing
		if s.isCreature(k, sl) && s.ctrlRoleF2(k, kop.F2TgtPlayerCtrl, s.perms[sl].ctrl) {
			ans = true
			pw := s.power(k, sl)
			for m := s.occ; m.any(); m = m.rest() {
				o2 := m.first()
				if o2 != sl && s.isCreature(k, o2) && s.ctrlRoleF2(k, kop.F2TgtPlayerCtrl, s.perms[o2].ctrl) && s.power(k, o2) > pw {
					ans = false
					break
				}
			}
		}
	case kop.W7CostBlue:
		// gorge ManaCostPartialBlue reads the printed cost; a face-down
		// permanent has none (CR 708.2)
		c := s.cardAt(sl)
		ans = c != kop.NoCard && !s.faceDown(k, sl) && k.t.Cards[c].Cost.HasColour(1)
	case kop.W7IsCommander:
		// a permanent whose object is its owner's commander (CR 903.3); no
		// object is a commander outside a Commander game
		o := s.perms[sl].obj
		p := s.objs[o].owner()
		ans = s.cfg&cfgCommander != 0 && s.pl[p].cmd == o
	case kop.W7NotBottomLibrary:
		ans = true // a battlefield permanent is no library's bottom card
	case kop.W7CrewedBySrc:
		// the source crewed this Vehicle this turn (gorge CrewedBySourceThisTurn);
		// no source fails closed
		ans = src != NoSlot && s.srcMarked(smCrewed, s.perms[sl].obj, src)
	}
	return ans != DebugWord9Flip
}

// wordCard3 is the sub-kinds on a card off the battlefield (fd: face down): withWarp reads
// the printed keyword in any zone, face down or not (gorge objectHasKeyword
// reads the printed face); the other two read nothing there (gorge: a
// battlefield creature, a crewed Vehicle).
//
//go:noinline
func (s *State) wordCard3(k *prep, fl *kop.Filter, c CardID, fd bool) bool {
	ans := false
	switch fl.PredN & 0xff {
	case kop.W7WithWarp:
		ans = c != kop.NoCard && k.t.Cards[c].HasWarp
	case kop.W7WithDisguise:
		ans = c != kop.NoCard && k.t.Cards[c].MorphFamily == kop.MorphDisguise
	case kop.W7WithFlashback:
		ans = c != kop.NoCard && k.t.Cards[c].Keywords&kop.KwFlashback != 0
	case kop.W7FaceUpDisguise:
		ans = c != kop.NoCard && !fd && k.t.Cards[c].MorphFamily == kop.MorphDisguise
	case kop.W7IsCommander:
		// off the battlefield the word sees only a CardID: no card is a
		// commander there (CR 903.3)
	case kop.W7CostBlue:
		ans = c != kop.NoCard && !fd && k.t.Cards[c].Cost.HasColour(1)
	case kop.W7NoAbilities:
		// gorge noAbilitiesPermanent over the printed face (a face-down
		// card here is a battlefield permanent, which kgen never lowers)
		ans = c != kop.NoCard && k.t.Cards[c].NoAbilities
	}
	return ans != DebugWord9Flip
}

// exileCardCands appends the exiled cards spec t accepts to out[:n], player
// 0's exile list then player 1's, each in list order (gorge target_legal.go
// walks the inferred zone per seat in zone order; CR 406.1). Cold: only a
// TgtCard slot whose filter zone is exile reaches it.
//
//go:noinline
func (s *State) exileCardCands(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, out *[MaxOptions]uint8, n int) int {
	for pp := 0; pp < int(s.nSeats()); pp++ {
		if s.out(uint8(pp)) {
			continue
		}
		pl := &s.pl[pp]
		for i := 0; i < int(pl.exileN); i++ {
			o := pl.exile[i]
			if s.targetOK(k, t, p, src, self, uint8(o)) && n < MaxOptions-1 {
				out[n] = uint8(o)
				n++
			}
		}
	}
	return n
}

// prepWord9 sets the prep of this code's reads: the cards whose abilities'
// filters read W7CrewedBySrc record the crews they pay (genPrep.crewerRec),
// and a PredWasCastEnter with the ManaSpent-EQ0 bit keeps each spell's
// recorded spend (anySpent, anyHist: spentOfSpell).
func prepWord9(t *kop.Table, g *genPrep) {
	for i := range t.Filters {
		if f := &t.Filters[i]; f.Pred == kop.PredWasCastEnter && f.PredN&wasCastSpentZero != 0 {
			g.anySpent, g.anyHist = true, true
			break
		}
	}
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			a := &c.Abilities[j]
			reads := chainWord7(t, a.Trig.Filter, kop.W7CrewedBySrc) || chainWord7(t, a.Trig.Filter2, kop.W7CrewedBySrc)
			for _, ts := range a.Targets {
				reads = reads || chainWord7(t, ts.Filter, kop.W7CrewedBySrc)
			}
			for _, in := range a.Prog {
				reads = reads || chainWord7(t, in.F, kop.W7CrewedBySrc)
			}
			if reads {
				if g.crewerRec == nil {
					g.crewerRec = make([]bool, len(t.Cards))
				}
				g.crewerRec[i] = true
				g.anySrcMark = true
				if g.crewBy == nil {
					// the crew pick's one guard (cast.go: crewBy != nil) now
					// also opens for a crewer-side reader; no vehicle is
					// flagged by it (noteCrew reads crewBy[card])
					g.crewBy = make([]bool, len(t.Cards))
				}
			}
		}
	}
}

// chainWord7 reports whether filter f's Or chain holds the PredWord7
// sub-kind n.
func chainWord7(t *kop.Table, f kop.FilterID, n int16) bool {
	for i := 0; f != 0 && int(f) < len(t.Filters) && i < len(t.Filters); i++ {
		if fl := &t.Filters[f]; fl.Pred == kop.PredWord7 && fl.PredN&0xff == n {
			return true
		}
		f = t.Filters[f].Or
	}
	return false
}

// crewerRecorded reports whether the creature at crewer is a card whose
// ability reads W7CrewedBySrc (so the crews it pays are recorded).
func (s *State) crewerRecorded(k *prep, crewer Slot) bool {
	if k.gen == nil || k.gen.crewerRec == nil || crewer == NoSlot || !s.occ.has(crewer) {
		return false
	}
	c := s.cardAt(crewer)
	return int(c) < len(k.gen.crewerRec) && k.gen.crewerRec[c]
}

// bottomLibrary reports whether o is the last card of its owner's library.
func (s *State) bottomLibrary(o Obj) bool {
	if o == NoObj || s.objs[o].zone != kop.ZoneLibrary {
		return false
	}
	pl := &s.pl[s.objs[o].owner()]
	return pl.libLo < pl.libHi && pl.lib[pl.libHi-1] == o
}

// sharesCrTrigObj is kop.W7SharesCrTrigObj for object o: o shares a creature
// type with the current entry's triggering object, each side read where it
// now is (gorge sharesCreatureTypeWith over sharesTypeReferents' TriggerCard:
// the battlefield's current types, else the printed card). No entry or no
// triggering object fails closed.
func (s *State) sharesCrTrigObj(k *prep, o Obj) bool {
	e := s.curEntry()
	if e == nil || e.trigObj == NoObj {
		return false
	}
	rm, ra := s.crTypesOf(k, e.trigObj)
	om, oa := s.crTypesOf(k, o)
	return s.sharesCr(om, oa, rm, ra) != DebugSharesCrTrigObjFlip
}

// DebugSharesCrTrigObjFlip is the mutation switch: every
// kop.W7SharesCrTrigObj answer flips. Test-only and cold.
var DebugSharesCrTrigObjFlip bool

// attachedToWord is kop.W7AttachedTo on the battlefield permanent at sl:
// it is attached to a battlefield permanent of the class bits names (a
// creature, a land, else any) that meets the qualifier: AttToOther, not the
// source slot src (no source excludes nothing, as gorge's Other with no
// source); AttToYouCtrl, controlled by ctrl (gorge attachedToArg, the
// qualifier read against the bearer).
func (s *State) attachedToWord(k *prep, bits int16, sl Slot, ctrl uint8, src Slot) bool {
	h := s.perms[sl].attached
	if h == NoSlot || !s.occ.has(h) {
		return false
	}
	if bits&kop.AttToCreature != 0 && !s.isCreature(k, h) || bits&kop.AttToLand != 0 && s.typesGen(k, h)&kop.TLand == 0 {
		return false
	}
	if bits&kop.AttToOther != 0 && h == src || bits&kop.AttToYouCtrl != 0 && s.perms[h].ctrl != ctrl {
		return false
	}
	return true
}
