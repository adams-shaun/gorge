package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StatStackKw: AffectedZone$ Stack keyword grants to the
// spells the static's controller casts: Cascade (Quandrix, the Proof), read
// live when the cast trigger queues (rules cascade.go queueCascadeTriggers);
// Improvise (Ironheart, Inspiring Statuary), Convoke (Chief Engineer,
// We Ride at Dawn, Invasion of Segovia) and Casualty:<n> (Silverquill),
// read wherever the printed CardDef.Improvise / Convoke / Casualty are
// (tapPays, hasConvoke, isImprovise, casualtyN). A grant beside the other
// tap-paid keyword, or on an {X} cost, poses the mixed or pre-X announcement;
// a Harmonize or alternative-cost route stalls unmodelled (kw_convoke.go).
// gorge derives the spell under the
// stack-zone override (keywordmay.go stackKeywordPossibleH), so the kernel
// asks the live statics at the same moment. gorge offers a tap-paid or
// casualty cast only against a floating pool, which the canonical vocabulary
// never builds (mana abilities are not actions); the stackkw grant lockstep
// floats one with Iridescent Tiger's ETB mana.
// Affinity grants are an ordinary ReduceCost (kgen).
//
// Every caller is behind genPrep.anyStackKw, so FDN and BLB never get here.

// DebugStackKwOff makes stackKwN answer 0 (the oracle's
// mutation test: the grants are ignored, so the lockstep must diverge).
var DebugStackKwOff bool

// DebugStackGrantIgnoreGate makes stackKwN read every gate as true (the
// oracle's mutation test: a gated grant then reaches a cast its condition
// excludes, so the option lists diverge).
var DebugStackGrantIgnoreGate bool

// prepStackKw sets the StatStackKw gates: a Cascade grant runs the cascade
// trigger queue, an Improvise grant opens the convoke paths (anyConvoke), a
// Casualty grant the casualty offer (p.casualty); one restricted to spells
// cast from hand reads the cast provenance (stackEntry.prov, castemit).
// The grant may live on an EMBLEM (Ral, Crackling Wit's "Instant and sorcery
// spells you cast have storm"), so the walk reads the emblem statics too --
// stackKwN's eachStatic already consults them, and skipping them here kept
// genPrep.anyStorm false and the granted-storm cast trigger unqueued
// (std-blb pair 1309, Valley Rally under the Ral emblem).
func prepStackKw(t *Table, g *genPrep, p *prep) {
	for i := range t.Cards {
		if t.Cards[i].Storm {
			// the printed K:Storm trigger (CR 702.40a): its count reads the
			// history bank, its copies choose new targets and a copy of a
			// permanent spell becomes a token (kw_storm.go,
			// copy_permanentspell.go)
			g.anyHist, g.anyCopyElect, g.anyPermCopy = true, true, true
		}
		if t.Cards[i].Conspire {
			g.anyConspire = true // the printed keyword offers the conspired cast (kw_conspire.go)
		}
		if t.Cards[i].SplitSecond {
			g.anySplitSecond = true // the printed keyword (kw_splitsecond.go)
		}
	}
	if !tableHasStatic(t, kop.StatStackKw) {
		return
	}
	g.anyStackKw = true
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			stackKwGates(&t.Cards[i].Abilities[j], g, p)
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			stackKwGates(&t.Emblems[i].Abilities[j], g, p)
		}
	}
}

// stackKwGates opens the genPrep/prep gates one live StatStackKw static needs.
func stackKwGates(a *kop.Ability, g *genPrep, p *prep) {
	if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatStackKw {
		return
	}
	if a.Static.Zone == kop.ZoneHand || a.Static.Zone == kop.ZoneAny {
		// the push records its origin zone (CastHand's fromHand read; the
		// StatStackKw-only ZoneAny marker means !wasCastFromYourHand)
		g.anyCastProv = true
	}
	if a.Static.Zone == kop.ZoneExile {
		// CastExile reads the push's origin zone (stackKwN,
		// castCameFromExile) and the cast ring, so both must record it
		g.anyCastProv = true
		g.castRingOn = true
	}
	switch a.Static.A {
	case kop.StackKwCascade:
		g.anyCascade = true
	case kop.StackKwImprovise, kop.StackKwConvoke:
		g.anyConvoke = true
	case kop.StackKwDelve:
		g.anyDelve = true
	case kop.StackKwCasualty:
		p.casualty = true
	case kop.StackKwLifelink:
		g.anyStackLife = true
	case kop.StackKwSplitSecond:
		g.anySplitSecond = true
	case kop.StackKwConspire:
		g.anyConspire, g.anyCopyElect, g.anyPermCopy = true, true, true // its copy chooses new targets; a granted spell may be a permanent
	case kop.StackKwDemonstrate:
		g.anyDemoGrant = true
		g.anyCopyElect, g.anyPermCopy = true, true // its copies choose new targets, and a granted spell may be a permanent
	case kop.StackKwStorm:
		g.anyStorm, g.anyHist = true, true         // the count reads the history bank
		g.anyCopyElect, g.anyPermCopy = true, true // its copies choose new targets, and a granted spell may be a permanent
	}
}

// stackKwN counts the live StatStackKw statics of keyword code that the
// caster controls and whose filter matches spell card c (object o), and
// returns the first match's B (the casualty threshold);
// fromHand answers a wasCastFromYourHand restriction. Walked like
// mayPlayStatics: the permanents that have not lost their abilities, then
// emblems. A spell on the stack grants nothing (CR 113.6) EXCEPT its own
// EffectZone$ Stack self grant (Bloodbraid Marauder), read off the stack
// entry below.
func (s *State) stackKwN(k *prep, code int16, c CardID, o Obj, caster uint8, fromHand bool) (n int, b int16) {
	if DebugStackKwOff {
		return 0, 0
	}
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatStackKw || a.Static.A != code || st.ctrl != caster {
			return true
		}
		if a.Static.Zone == kop.ZoneStack {
			return true // an EffectZone$ Stack self grant functions only on the stack: stackKwSelfN reads it
		}
		if a.Static.Zone == kop.ZoneHand && !fromHand {
			return true
		}
		if a.Static.Zone == kop.ZoneAny && fromHand {
			return true // !wasCastFromYourHand
		}
		if a.Static.Zone == kop.ZoneExile && !s.castCameFromExile(o, caster) {
			return true
		}
		// a Condition$ / IsPresent$ / CheckSVar$ gate (kgen lower_stackkw.go);
		// a cast-time caller hides the in-flight cast's push from the cast
		// ring first (hideInflightCast), as gorge's hasKeywordPriorCasts does
		if a.Static.Cond2 != 0 && !DebugStackGrantIgnoreGate && !s.statCond(k, st) {
			return true
		}
		if s.matchSpellCard(k, a.Static.Filter, c, caster, caster, o, st.sl, st.emb) {
			if n == 0 {
				b = a.Static.B
			}
			n++
		}
		return true
	})
	if sn, sb := s.stackKwSelfN(k, code, c, o, caster); sn > 0 {
		if n == 0 {
			b = sb
		}
		n += sn
	}
	return n, b
}

// stackKwSelfN counts the EffectZone$ Stack self grants on the spell being
// cast: a printed StatStackKw static of the card itself, read off its stack
// entry (gorge applies it while the source is on the stack,
// chars.StaticZoneAdmitsStatic). The spell's controller is its caster, so the
// static's condition and filter read against the caster; a gate is read like
// any other stack grant's (DebugStackGrantIgnoreGate, hideInflightCast).
func (s *State) stackKwSelfN(k *prep, code int16, c CardID, o Obj, caster uint8) (n int, b int16) {
	if o == NoObj || s.objs[o].zone != kop.ZoneStack {
		return 0, 0
	}
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatStackKw || a.Static.A != code || a.Static.Zone != kop.ZoneStack {
			continue
		}
		st := statSrc{a: a, sl: NoSlot, ctrl: caster, obj: o}
		if a.Static.Cond2 != 0 && !DebugStackGrantIgnoreGate && !s.statCond(k, st) {
			continue
		}
		if s.matchSpellCard(k, a.Static.Filter, c, caster, caster, o, NoSlot, 0) {
			if n == 0 {
				b = a.Static.B
			}
			n++
		}
	}
	return n, b
}

// castCameFromExile reports whether the cast of o by caster came from exile:
// the proposed card is still in exile at an offer; a spell already on the
// stack reads its entry's provenance (stackEntry.prov, which survives the
// cast-ring hide); anything else reads the cast ring's origin (noteCast). A
// cast from anywhere else reads false.
func (s *State) castCameFromExile(o Obj, caster uint8) bool {
	if o == NoObj {
		return false
	}
	if s.objs[o].zone == kop.ZoneExile {
		return true
	}
	// a cast-time caller hides the in-flight push from the ring
	// (hideInflightCast), so the stack entry is the authoritative origin for
	// a spell that is already on the stack
	if e := s.spellEntryOf(o); e != nil && e.ctrl == caster && s.objs[o].zone == kop.ZoneStack {
		return kop.Zone(e.prov&7) == kop.ZoneExile
	}
	ring := s.castPushes(caster)
	for i := len(ring) - 1; i >= 0; i-- {
		if ring[i].obj == o+1 {
			return ring[i].prov&castFromMask == castFromExile
		}
	}
	return false
}

// hideInflightCast removes e's own push from the cast ring while a stack
// grant's condition is read, so a "first spell each turn" Count$ThisTurnCast
// gate sees the casts PRIOR to the in-flight one (gorge hasKeywordPriorCasts:
// trigger_granted.go). It returns the saved push count (0 when the latest
// push is not e's, so restoreInflightCast is a no-op).
func (s *State) hideInflightCast(e *stackEntry) uint8 {
	if e == nil || e.kind != seSpell {
		return 0
	}
	p := e.ctrl
	n := s.g.hist[p][histCastN]
	if n == 0 || Obj(s.cold.castRing[int(p)*castRingN+int(n)-1].obj-1) != e.obj {
		return 0
	}
	s.g.hist[p][histCastN] = n - 1
	return n
}

// restoreInflightCast puts back the push hideInflightCast removed.
func (s *State) restoreInflightCast(p uint8, saved uint8) {
	if saved != 0 {
		s.g.hist[p][histCastN] = saved
	}
}

// grantImprovise: a live StatStackKw Improvise grant reaches p's spell card
// c (object o). Behind genPrep.anyStackKw.
func (s *State) grantImprovise(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	if k.gen == nil || !k.gen.anyStackKw {
		return false
	}
	n, _ := s.stackKwN(k, kop.StackKwImprovise, c, o, p, fromHand)
	return n > 0
}

// creditOfferLive reports a convoke, improvise or delve credit live for
// p's offered cast of card c (object o): the printed keyword or a granted
// one the announcement reaches. At the offer the candidate is still in its
// zone, not on the stack, so the grant read stackKwN makes answers a
// wasCast Affected$ clause through the spell-entry reader (wasCastSpell)
// and finds none: a grant whose filter carries it (Ironheart, Clever
// Champion's "Noncreature spells you cast have improvise") is missed. The
// planner read gorge's potential plan makes for the cast (potentialModeCredit
// over hasCastConvoke and friends, derivedWith's stack override) matches that
// clause for the offered cast -- for ANY live grant, the grant's controller
// standing in as the filter's "you" and no caster check: an opponent's
// Ironheart over p's exiled nonland card is a credit to that read too. So the
// grant loop here answers it the way the CantBeCast offer read does
// (matchSpellCardOffer: the origin the cast would come from) per live static,
// without the caster filter. The keyword reads above read the card's own
// controller (an exiled card's owner) too, never the caster: p's Ironheart
// over the other seat's exiled Borough Backup (Black Widow's dig grant) is
// no credit to gorge's read (its YouCtrl names the owner), so the cast is
// priced and offered.
func (s *State) creditOfferLive(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	q := s.creditReader(p, o)
	if s.hasConvoke(k, q, c, o, fromHand) || s.hasCastDelve(k, q, c, o, fromHand) || s.isImprovise(k, q, c, o, fromHand) {
		return true
	}
	if k.gen == nil || !k.gen.anyStackKw || s.objs[o].zone == kop.ZoneBattlefield {
		return false // a permanent on the battlefield is never a spell being cast (kw_delve.go)
	}
	live := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if live || a.Kind != kop.AbStatic || a.Static.Kind != kop.StatStackKw {
			return true
		}
		switch a.Static.A {
		case kop.StackKwConvoke, kop.StackKwImprovise, kop.StackKwDelve:
		default:
			return true
		}
		if a.Static.Zone == kop.ZoneStack {
			return true // an EffectZone$ Stack self grant functions only on the stack: stackKwSelfN reads it
		}
		if a.Static.Zone == kop.ZoneHand && !fromHand {
			return true
		}
		if a.Static.Zone == kop.ZoneAny && fromHand {
			return true // !wasCastFromYourHand
		}
		if a.Static.Zone == kop.ZoneExile && !s.castCameFromExile(o, p) {
			return true
		}
		if a.Static.Cond2 != 0 && !DebugStackGrantIgnoreGate && !s.statCond(k, st) {
			return true
		}
		// the static's own controller is the filter's "you": gorge's derived
		// read resolves it against the offered card, never the caster (the
		// dig grant's cast of another seat's exiled card, pair 341). The
		// grant's "spells you cast" (kgen keeps it as the static's
		// controller, not a filter clause) reads the card's own controller,
		// an exiled card's owner: Dazzling Theater does not credit the
		// other seat's exiled Fear of Burning Alive.
		if o != NoObj && s.objs[o].owner() != st.ctrl {
			return true
		}
		if s.matchSpellCardOffer(k, a.Static.Filter, c, st.ctrl, st.ctrl, o, st.sl, st.emb) {
			live = true
		}
		return true
	})
	return live
}

// grantDelve: a live StatStackKw Delve grant reaches p's spell card c
// (object o). Behind genPrep.anyDelve.
func (s *State) grantDelve(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	if k.gen == nil || !k.gen.anyDelve {
		return false
	}
	n, _ := s.stackKwN(k, kop.StackKwDelve, c, o, p, fromHand)
	return n > 0
}

// grantConspire: a live StatStackKw Conspire grant reaches p's spell card c
// (object o). Behind genPrep.anyConspire.
func (s *State) grantConspire(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	if k.gen == nil || !k.gen.anyConspire {
		return false
	}
	if k.t.Cards[c].Conspire {
		return true // the printed keyword
	}
	n, _ := s.stackKwN(k, kop.StackKwConspire, c, o, p, fromHand)
	return n > 0
}

// grantConvoke: a live StatStackKw Convoke grant reaches p's spell card c
// (object o). Behind genPrep.anyStackKw.
func (s *State) grantConvoke(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	if k.gen == nil || !k.gen.anyStackKw {
		return false
	}
	n, _ := s.stackKwN(k, kop.StackKwConvoke, c, o, p, fromHand)
	return n > 0
}

// tapPays is CardDef.TapPays for p's spell card c (object o): printed
// convoke or improvise, or a granted one (the cast announces tapped
// permanents toward its mana cost). fromHand: the spell is cast from hand.
func (s *State) tapPays(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	return s.hasConvoke(k, p, c, o, fromHand) || s.isImprovise(k, p, c, o, fromHand)
}

// hasConvoke is CardDef.Convoke for p's spell card c (object o): printed or
// granted convoke (the tap-paid permanents are creatures). A grant beside an
// improvise source is unmodelled, so kw_convoke.go stalls it rather than
// posing one keyword's announcement.
func (s *State) hasConvoke(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	return k.t.Cards[c].Convoke || s.grantConvoke(k, p, c, o, fromHand)
}

// isImprovise is CardDef.Improvise for p's spell card c (object o): printed
// or granted improvise (the tap-paid permanents are artifacts, not creatures).
func (s *State) isImprovise(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	return k.t.Cards[c].Improvise || s.grantImprovise(k, p, c, o, fromHand)
}

// casualtyN is the casualty threshold of p's spell card c (object o): the
// printed fixed N, else the first matching granted Casualty:<n> (gorge
// casualtySpec: the first parseable keyword line, printed before granted);
// 0 = none. Behind prep.casualty.
func (s *State) casualtyN(k *prep, p uint8, c CardID, o Obj, fromHand bool) int {
	if n := int(k.t.Cards[c].Casualty); n != 0 {
		return n
	}
	if k.gen == nil || !k.gen.anyStackKw {
		return 0
	}
	_, b := s.stackKwN(k, kop.StackKwCasualty, c, o, p, fromHand)
	return int(b)
}
