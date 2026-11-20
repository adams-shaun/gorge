package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Trigger-event and keyword classes 19-62: cold,
// out-of-line arms.

// protPrinted is the permanent at sl's printed protection-from-colour
// colours (CardDef.ProtColors; CR 702.16), none once it has lost its
// abilities (gorge protectedFrom reads the derived keywords), and the
// colours a chosen-colour grant gave it until end of turn (cgProtOf).
//
//go:noinline
func (s *State) protPrinted(k *prep, sl Slot) kop.ColorMask {
	c := k.t.Cards[s.cardAt(sl)].ProtColors
	if c != 0 && s.genLost(k, sl) {
		c = 0
	}
	if k.gen.anyColGrant {
		c |= s.colProtOf(k, sl) // static_colourprot.go
	}
	return c
}

// protFrom reports whether permanent sl has protection from a damage
// source (slot srcSl, NoSlot off the battlefield; card srcCard; colours
// col): from one of its colours (protColFrom), or a K:Protection:<spec>
// quality it matches (CardDef.ProtFrom or a granted one; gorge
// sourceHasQuality).
func (s *State) protFrom(k *prep, sl, srcSl Slot, srcCard CardID, col kop.ColorMask, combat bool) bool {
	return (s.protColFrom(k, sl, col) || k.gen.anyProtF && s.protQuality(k, sl, srcSl, srcCard)) && !s.noPrev(k, combat) // CR 615.12
}

// protQuality: sl's printed K:Protection:<spec> filter matches the source
// (the permanent at srcSl, else card srcCard as a spell or a departed
// source), or a granted one does (protGranted); the printed one is none
// once sl lost its abilities.
//
//go:noinline
func (s *State) protQuality(k *prep, sl, srcSl Slot, srcCard CardID) bool {
	return s.protQualityAt(k, sl, srcSl, srcCard, false)
}

// protQualityAt is protQuality; sba marks the Aura state-based check
// (turn.go), where a granted protection's exception spares its source (the
// attached Auras and Equipment a Benevolent Blessing leaves, CR 702.16).
//
//go:noinline
func (s *State) protQualityAt(k *prep, sl, srcSl Slot, srcCard CardID, sba bool) bool {
	if f := k.t.Cards[s.cardAt(sl)].ProtFrom; f != 0 && !s.genLost(k, sl) && s.protMatch(k, f, sl, srcSl, srcCard) {
		return true
	}
	return k.gen.anyProtGrant && (s.protOvF(k, sl, srcSl, srcCard, sba) || s.protGranted(k, sl, srcSl, srcCard, sba) || s.protYours(k, sl, srcSl, srcCard))
}

// protOvF: an override record's ProtFrom quality, granted to sl (a Pump's or
// an Animate's granted protection, kop.Override.ProtFrom), matches the
// source (CR 702.16; Figure of Fable's protection from each of your
// opponents, Eldritch Immunity's from each color). A grant is a layer-6
// effect, so it does not survive a later loss of sl's abilities (the rule
// protGranted uses). Cold: protQuality calls it behind
// genPrep.anyProtGrant (an Override record with a ProtFrom arms it too).
//
//go:noinline
func (s *State) protOvF(k *prep, sl, srcSl Slot, srcCard CardID, sba bool) bool {
	lts, lost := s.lossAt(k, sl)
	hit := false
	s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
		if o.ProtFrom == 0 || lost && ts < lts {
			return
		}
		if s.protOvMatch(k, o.ProtFrom, sl, srcSl, srcCard) {
			hit = true
		}
	})
	return hit
}

// grantedProtColKw is the oracle projection's keyword bits for the
// permanent at sl's granted protection-from-colour qualities (a Continuous
// static's or an override record's ProtFrom colour clauses, CR 702.16):
// gorge lists each as a "Protection from <colour>" keyword, and black and
// red have kernel bits (KwProtBlack, KwProtRed); white, blue and green have
// none, and the oracle drops the spelling. A quality with any clause beyond
// a plain colour mask (gorge lists it by its spec, the oracle drops the
// "Protection:" spellings) projects nothing. A colour the card prints
// protection from is dropped, as the oracle drops the printed spelling.
// Cold: projKw calls it behind anyProtGrant.
//
//go:noinline
func (s *State) grantedProtColKw(k *prep, sl Slot) kop.KwMask {
	lts, lost := s.lossAt(k, sl)
	var col kop.ColorMask
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		pf := staticProtFrom(k, a)
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous && a.Static.Kind != kop.StatOverride || pf == 0 {
			return true
		}
		if lost && s.grantTs(st) < lts {
			return true
		}
		if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			return true
		}
		col |= protColKwOf(k, pf)
		return true
	})
	s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
		if o.ProtFrom == 0 || lost && ts < lts {
			return
		}
		col |= protColKwOf(k, o.ProtFrom)
	})
	return protColourKw(col &^ k.t.Cards[s.cardAt(sl)].ProtColors)
}

// protColKwOf is a protection quality filter's colour-projection colours:
// the colours of its colour clauses (CR 702.16), a clause beyond
// a plain colour mask (a type, controller or value term: gorge lists that
// term by its spec) adds none (Pristine Angel's artifacts beside each color).
func protColKwOf(k *prep, f kop.FilterID) kop.ColorMask {
	var kw kop.ColorMask
	for ; f != 0; f = k.t.Filters[f].Or {
		fl := &k.t.Filters[f]
		if fl.Types != 0 || fl.NotTypes != 0 || fl.Sub != 0 || fl.NotSub != 0 || fl.NotSubs != [4]uint8{} ||
			fl.Ctrl != 0 || fl.Owner != 0 || fl.Flags != 0 || fl.Flags2 != 0 || fl.NotColors != 0 ||
			fl.Pred != 0 || fl.PredV != (kop.ValueRef{}) || fl.Each || fl.Zone != 0 || fl.CtrlTgt != 0 {
			continue // another term of the grant (gorge lists it by its spec)
		}
		kw |= fl.Colors
	}
	return kw
}

// protOvMatch: the protection quality filter f matches the source, read for
// a permanent controlled as sl is. A live battlefield source is read as a
// permanent; a resolving or probed spell as its card with ITS controller as
// the owner (gorge judges a live source object's controller against a
// player-relative quality, protection.go's player-relative arm). A departed
// source with no object (a ceased token) matches nothing: gorge's arm needs g.Obj(source) != nil.
func (s *State) protOvMatch(k *prep, f kop.FilterID, sl, srcSl Slot, srcCard CardID) bool {
	p := s.perms[sl].ctrl
	if srcSl != NoSlot && s.occ.has(srcSl) {
		return s.matchPermGen(k, f, srcSl, p, NoSlot, layerFull)
	}
	if e := s.curEntry(); e != nil && e.kind == seSpell && e.card == srcCard {
		return s.matchCardGen(k, f, srcCard, e.ctrl, p, e.obj)
	}
	if s.g.probe.kind == probeSpell && s.g.probe.card == srcCard {
		return s.matchCardGen(k, f, srcCard, s.g.probe.ctrl, p, NoObj)
	}
	// an ability whose source card left the battlefield (Seven-Tail Mentor's
	// death trigger) still has a source object in its new zone: gorge reads its
	// characteristics there (CR 702.16b); a token that ceased to exist has
	// none, which matchCardGen reads as no match
	if e := s.curEntry(); e != nil && e.kind != seSpell && e.card == srcCard && e.obj != NoObj {
		return s.matchCardGen(k, f, srcCard, e.ctrl, p, e.obj)
	}
	return false
}

// protMatch: the protection quality filter f matches the source, read for
// a permanent controlled as sl is.
func (s *State) protMatch(k *prep, f kop.FilterID, sl, srcSl Slot, srcCard CardID) bool {
	p := s.perms[sl].ctrl
	if srcSl != NoSlot && s.occ.has(srcSl) {
		return s.matchPermGen(k, f, srcSl, p, NoSlot, layerFull)
	}
	if x := s.protSpellXMV(k, srcCard); x != 0 {
		ctx := s.ctxCur()
		ctx.xmv = x
		return s.matchCardChain(k, f, srcCard, p, p, NoObj, &ctx)
	}
	return s.matchCardGen(k, f, srcCard, p, p, NoObj)
}

// protSpellXMV is what the announced X of spell srcCard adds to the mana
// value a protection quality reads on it (CR 202.3e, 601.2b: X is announced
// before the targets; gorge protectionSourceMV): the X a feasibility sweep
// is trying (probe.x, xAnswerable and xBoundPayable), else the X of the
// spell being built or resolved. 0 for a card with no {X} or any other source.
func (s *State) protSpellXMV(k *prep, srcCard CardID) int {
	nx := int(k.cards[srcCard].cost.X)
	if nx == 0 {
		return 0
	}
	if p := &s.g.probe; p.kind == probeSpell && p.card == srcCard && p.x != 0 {
		return (int(p.x) - 1) * nx
	}
	if e := s.curEntry(); e != nil && e.kind == seSpell && e.card == srcCard {
		return int(e.x) * nx
	}
	return 0
}

// protXBoundTarget: some target slot of a can take a permanent or card, so
// a protection quality that reads the spell's announced X (protSpellXMV) can
// leave it without a candidate at some X. A slot that takes only players has
// no protection quality to read (gorge's xBoundTargetPayable judges an X cost
// only for a target bounded by X, and its player slots carry no such bound:
// Mind Shatter's "target player discards X cards at random" is payable at any
// X its mana pays).
func protXBoundTarget(k *prep, a *kop.Ability) bool {
	if k.gen == nil || !k.gen.anyProtF {
		return false
	}
	for i := 0; i < int(a.NTargets); i++ {
		if a.Targets[i].Kind != kop.TgtPlayer {
			return true
		}
	}
	return false
}

// protGranted: a Continuous static's ProtFrom quality, granted to sl,
// matches the source (CR 702.16; Sword of Wealth and Power's instants and
// sorceries). A grant is a layer-6 effect, so it does not survive a later
// loss of sl's abilities (the rule kwGen uses). Cold: protQuality calls it
// behind genPrep.anyProtGrant.
//
//go:noinline
func (s *State) protGranted(k *prep, sl, srcSl Slot, srcCard CardID, sba bool) bool {
	lts, lost := s.lossAt(k, sl)
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		pf := staticProtFrom(k, a)
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous && a.Static.Kind != kop.StatOverride || pf == 0 && !a.Static.ProtChosen {
			return true
		}
		if lost && s.grantTs(st) < lts {
			return true
		}
		if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			return true
		}
		if !(pf != 0 && s.protMatch(k, pf, sl, srcSl, srcCard) || a.Static.ProtChosen && s.protChosenHit(k, st.sl, srcSl, srcCard)) {
			return true
		}
		if sba && !DebugProtectionNoWardException && a.Static.ProtExc != 0 && srcSl != NoSlot && s.occ.has(srcSl) &&
			s.matchPermGen(k, a.Static.ProtExc, srcSl, st.ctrl, st.sl, layerFull) {
			return true // the grant's exception spares this attachment (CR 702.16)
		}
		hit = true
		return false
	})
	return hit
}

// DebugProtectionNoWardException is the mutation hook of the granted
// protection exception (TestProtectionWardExceptionMutation): set, a Ward
// Aura's own protection is no longer spared at the Aura state-based check.
var DebugProtectionNoWardException bool

// staticProtFrom is the protection quality filter a static grants: a Continuous
// static's ProtFrom, or the ProtFrom of the Override record a StatOverride
// static applies (Hexdrinker's levels 3-7: protection from instants beside
// its 4/4); 0 for any other static.
func staticProtFrom(k *prep, a *kop.Ability) kop.FilterID {
	switch {
	case a.Kind != kop.AbStatic:
		return 0
	case a.Static.Kind == kop.StatContinuous:
		return a.Static.ProtFrom
	case a.Static.Kind == kop.StatOverride && a.Static.Override != 0:
		return k.t.Overrides[a.Static.Override-1].ProtFrom // StatOverride's index is 1-based
	}
	return 0
}

// protChosenHit: the source (the permanent at srcSl, else card srcCard as a
// spell or a departed source) has the colour the static source host chose
// (a grant of protection from the chosen colour); a host with no choice
// protects from nothing.
func (s *State) protChosenHit(k *prep, host, srcSl Slot, srcCard CardID) bool {
	ch := s.perms[host].chosen
	if ch > 4 {
		return false
	}
	if srcSl != NoSlot && s.occ.has(srcSl) {
		return s.colorsGen(k, srcSl)&(kop.ColW<<ch) != 0
	}
	return k.t.Cards[srcCard].Colors&(kop.ColW<<ch) != 0
}

// prepProtGrant arms the granted-protection reads (genPrep.anyProtGrant
// for a Continuous static with a ProtFrom quality; it also opens the
// targeting, block and attach sites through anyProtF) and the protection
// replacement bit the damage sites gate on.
func prepProtGrant(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if protGrantIn(t.Cards[i].Abilities) {
			g.anyProtGrant, g.anyProtF = true, true
			g.replBits[i] |= rbProtCol
		}
		if protAllGrantIn(t.Cards[i].Abilities, t) {
			g.replBits[i] |= rbProtAll // a granted protection from everything (Hexdrinker's level 8)
		}
		if ovProtFromIn(t.Cards[i].Abilities, t) {
			// the card's Animate or Pump grant record carries the protection:
			// the damage sites gate on rbProtCol the same way (anyProtF and
			// anyProtGrant are armed by the Table.Overrides scan in prepareGen)
			g.replBits[i] |= rbProtCol
		}
	}
	for i := range t.Emblems {
		if protGrantIn(t.Emblems[i].Abilities) {
			g.anyProtGrant, g.anyProtF = true, true
			g.anyRepl |= rbProtCol // an emblem has no replBits row: open the damage sites table-wide
		}
		if ovProtFromIn(t.Emblems[i].Abilities, t) {
			g.anyRepl |= rbProtCol
		}
	}
}

// protAllGrantIn reports whether any of the card's abilities is a static whose
// keywords, or whose Override record's, include protection from everything:
// the damage sites gate on rbProtAll, which a printed keyword arms (gen.go)
// and a granted one must too.
func protAllGrantIn(abs []kop.Ability, t *kop.Table) bool {
	for j := range abs {
		a := &abs[j]
		if a.Kind != kop.AbStatic {
			continue
		}
		if a.Static.Kw&kop.KwProtectionAll != 0 {
			return true
		}
		if a.Static.Kind == kop.StatOverride && a.Static.Override != 0 &&
			t.Overrides[a.Static.Override-1].AddKw&kop.KwProtectionAll != 0 { // StatOverride's index is 1-based
			return true
		}
	}
	return false
}

// protGrantIn reports whether any ability is a Continuous static granting
// protection from a quality.
func protGrantIn(abs []kop.Ability) bool {
	for j := range abs {
		if a := &abs[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && (a.Static.ProtFrom != 0 || a.Static.ProtChosen) {
			return true
		}
	}
	return false
}

// ovProtFromIn reports whether any of the card's abilities applies an
// Override record (a StatOverride static, an OpOverride or OpOverrideAll
// program step) whose record grants protection (kop.Override.ProtFrom).
func ovProtFromIn(abs []kop.Ability, t *kop.Table) bool {
	for j := range abs {
		a := &abs[j]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride && a.Static.Override != 0 &&
			t.Overrides[a.Static.Override-1].ProtFrom != 0 { // StatOverride's index is 1-based
			return true
		}
		for _, in := range a.Prog {
			// the programs' Override index C is 0-based (kop.OpOverride)
			if (in.Op == kop.OpOverride || in.Op == kop.OpOverrideAll) && int(in.C) < len(t.Overrides) &&
				t.Overrides[in.C].ProtFrom != 0 {
				return true
			}
		}
	}
	return false
}
