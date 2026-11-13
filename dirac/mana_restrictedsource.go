package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// StallRestrSrc: restricted mana a planner-tapped source leaves floating past
// the batches State.rb holds (more than maxRB of them, or 255 of one colour).
const StallRestrSrc = 64

func init() { stallNames[StallRestrSrc] = "restrsrc" }

// Restricted mana sources as planner units (gorge,
// CR 106.6, 601.2g-h, 602.2b).
//
// gorge's payment planner used to defer every RestrictValid$ mana ability
// (a scripted-prefix gap); it now counts the source as a census unit for
// exactly the payments its restriction admits (pay.PlanPaymentCostWithout's
// descriptor, RestrictValidMatches):
//   - a restricted fixed ability of a source with no census ability of its
//     own (Giada {W}, a Powerstone {C}, Kolvori {G}) or a restricted
//     Produced$ Any one (Gideon's Memorial) is appended with the
//     choice-shaped units, in battlefield order (rkExt);
//   - one beside a census ability (a Village's {W} beside {C}, Secluded
//     Courtyard's any colour beside {C}) joins the unit that ability has,
//     at the census position: the unit's colours widen (rkWiden);
//   - an evaluated amount (Helga's Amount$ X, her power) joins the
//     evaluated-amount layer, last (rkDyn).
//
// A payment is spell, activated ability (the source of the ability) or
// other (echo, cumulative upkeep, a triggered cost). The ward / unless
// mana window is unchanged: it activates every restricted source in
// battlefield order (windowRestr), and a pool holding restricted mana
// stays refused by the planner (State.rb). A reflected restricted ability
// (Plaza of Heroes) is still a census gap (planreflected.go refuses it).

// restrSrc record kinds.
const (
	rkWiden uint8 = 1 + iota // joins the card's own census unit
	rkExt                    // a unit of its own, with the choice-shaped ones
	rkDyn                    // an evaluated amount: the last layer
)

// DebugRestrictedSource is the lockstep mutation switch of TestKf24Restr*: 1 admits
// every restricted source for every payment, 2 none, 3 puts a widened unit
// (Secluded Courtyard) with the choice-shaped ones instead of its census
// position, 4 counts a restricted ability of a larger amount as one mana
// (Intrepid Stablemaster's two), 5 floats the mana of that alternative as
// ordinary mana. Zero in every real run.
var DebugRestrictedSource uint8

// payment classes (gorge pay.Purpose).
const (
	pdSpell uint8 = iota
	pdAct
	pdOther
	pdNone // no payment: the restricted sources count for nothing
)

// payDesc describes the payment a plan funds: for a spell the card, for an
// activated ability the source's card and (when it is a permanent) slot.
type payDesc struct {
	class uint8
	card  CardID
	src   Slot
	// og is the cast a spell payment funds, read by a provenance restriction
	// the zero value fails closed
	og restrOrigin
	// play is the action the payment funds when the search needs its timing
	// (grantPayDesc): a sorcery-speed play waits for a stack its prefix grew
	play    Action
	hasPlay bool
	// instant: the play may be made with a trigger on the stack (an
	// instant-speed activation, an instant or flash spell), so gorge's prefix
	// search, which leaves a tapped source's watcher trigger stacked, can fund
	// it (restrSrcAffords)
	instant bool
	// noPair prices the census alone: a chosen-colour widened pair stays its
	// census unit (mana_chosenprefix.go)
	noPair bool
}

// restrRec is one restricted mana ability of the table: card c's ability
// ab, planned through views[view] (rkDyn: views[view+n] for amount n).
type restrRec struct {
	card CardID
	ab   int8
	kind uint8
	view int32
}

// restrData is the table's restricted planner sources.
type restrData struct {
	recs  []restrRec
	views []pcard
	// ext: some record adds units of its own (rkExt, rkDyn) rather than
	// widening a census unit; nonSpell: some record may pay a payment that
	// is not a spell (AfManaNonSpell, or an activated restriction). Both
	// let restrSrcAffords answer without walking the battlefield.
	ext, nonSpell bool
	// wide: some record is a widened source whose restricted ability makes
	// more mana than its census one (pcard.wideN): a tap of it can overpay,
	// and what it overpays floats restricted
	wide bool
	// over: some record is an rkExt ability of more than one mana whose
	// source has no census alternative (Steelswarm Operator's activation
	// {U}{U}): a tap of it can overpay too, so its unit floats restricted
	// and the window payment takes payRestrX's route
	over bool
	// chosenPair: some record is a chosen-colour widened pair (Sunken
	// Citadel's {C}{C}), a census gap of gorge's prefix search
	// (mana_chosenprefix.go)
	chosenPair bool
}

// restrSrcAb: a is a plain tap mana ability with a spend restriction that
// gorge's planner counts per payment (not a reflected one).
func restrSrcAb(a *kop.Ability) bool {
	return a.Kind == kop.AbMana && a.ManaRestrict != 0 && !restrWideAnyMana(a) &&
		a.AFlags&^(kop.AfManaNonSpell|kop.AfManaDynamic|kop.AfManaChoice|kop.AfManaChosen) == 0 &&
		a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 &&
		(a.ManaColors&0x3f != 0 || a.AFlags&kop.AfManaChosen != 0)
}

// oneColour: m holds exactly one colour bit.
func oneColour(m kop.ColorMask) bool { return m != 0 && m&(m-1) == 0 }

// prepRestrSrc records card c's restricted planner sources: every restricted
// ability, so a card with two whose restrictions are disjoint by payment class
// (Steelswarm Operator's spell-only {U} and activation-only {U}{U}) plans the
// one its payment admits (restrViewW).
func (g *genPrep) prepRestrSrc(c *kop.CardDef, id CardID, pc *pcard) {
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.AFlags&kop.AfLatent != 0 || !restrSrcAb(a) {
			continue
		}
		if g.rs == nil {
			g.rs = &restrData{}
		}
		g.anyRestrSrc = true
		g.setFlag(id, gfRestrSrc)
		rec := restrRec{card: id, ab: int8(j), view: int32(len(g.rs.views))}
		n := max(a.ManaN, 1)
		switch {
		case a.AFlags&kop.AfManaDynamic != 0:
			rec.kind = rkDyn
			for n := 0; n <= maxDynMana; n++ {
				g.rs.views = append(g.rs.views, pcard{manaAb: int8(j), manaCol: a.ManaColors, manaN: uint8(n), manaTap: true, manaDyn: true})
			}
		case pc.manaAb >= 0 && pc.manaTap && !pc.manaMulti && pc.painCol == 0 &&
			(!pc.manaChosen || a.AFlags&kop.AfManaChosen != 0):
			rec.kind = rkWiden
			v := *pc
			if !a.ManaEach {
				// a ManaEach ability's view already split its first colour from
				// pcard.manaXv (prepEachMana); OR-ing the whole mask would count
				// the extra colours twice.
				v.manaCol |= a.ManaColors & 0x3f
			}
			switch {
			case pc.manaChosen && a.AFlags&kop.AfManaChosen != 0 && n > pc.manaN && a.AFlags&kop.AfManaChoice == 0:
				// a chosen-colour larger alternative beside the chosen census
				// ability (Sunken Citadel's {C}{C} beside its {C}): the pair's
				// larger type takes the permanent's chosen colour at runtime
				// (restrPairUnit, srcMaskAt)
				v.wideN, v.wideCol, v.wideChosen = n, a.ManaColors&kop.ColAny, true
				g.rs.wide, g.rs.chosenPair = true, true
			case n > pc.manaN && a.AFlags&kop.AfManaChoice == 0 &&
				(a.ManaColors&kop.ColAny == kop.ColAny ||
					!a.ManaEach && oneColour(a.ManaColors&kop.ColAny) && a.ManaColors&kop.ColAny == pc.manaCol&kop.ColAny):
				// several mana of one colour beside the census ability's one,
				// of any colour or of the census ability's own single colour
				// (Tablet of Discovery's {R}{R} beside {R}): gorge's planner
				// offers both alternatives (restrPairUnit)
				v.wideN, v.wideCol = n, a.ManaColors&kop.ColAny
				g.rs.wide = true
			}
			g.rs.views = append(g.rs.views, v)
		default:
			rec.kind = rkExt
			v := pcard{manaAb: int8(j), manaCol: a.ManaColors, manaN: n, manaTap: true}
			if a.AFlags&kop.AfManaChosen != 0 {
				v.manaChosen = true
			}
			if a.ManaEach {
				prepEachMana(a, &v)
			}
			firstSpellOnly := false
			for i := range g.rs.recs {
				rec := &g.rs.recs[i]
				if rec.card == id && rec.kind != rkDyn && c.Abilities[rec.ab].ManaRestrictAb == 0 {
					firstSpellOnly = true
				}
			}
			if n > 1 && a.ManaRestrictAb != 0 && firstSpellOnly {
				// the activation-only larger ability beside a spell-only first
				// one (Steelswarm Operator's {U}{U}): a tap of it can overpay,
				// so its unit floats restricted (plan) and the window payment
				// takes payRestrX's route. A sole restricted ability of more
				// than one mana keeps the old unit.
				v.restrFloat = true
				g.rs.over = true
			}
			g.rs.views = append(g.rs.views, v)
		}
		g.rs.recs = append(g.rs.recs, rec)
		g.rs.ext = g.rs.ext || rec.kind != rkWiden || g.rs.views[rec.view].wideN != 0
		g.rs.nonSpell = g.rs.nonSpell || a.AFlags&kop.AfManaNonSpell != 0 || a.ManaRestrictAb != 0
	}
}

// restrRecOf is card c's restricted-source record.
func (g *genPrep) restrRecOf(c CardID) *restrRec {
	for i := range g.rs.recs {
		if g.rs.recs[i].card == c {
			return &g.rs.recs[i]
		}
	}
	return nil
}

// payDescNow is the payment a plan taps for: the entry being built (a
// spell's card, an activated ability's source), else a spell when the
// caller excludes a hand card from the demand, else other.
func (s *State) payDescNow(exclude Obj) payDesc {
	if s.ctl == ctlBuild {
		switch e := &s.bld().e; e.kind {
		case seSpell:
			og := s.restrOriginOfObj(e.obj)
			if s.bld().bfl&bflPushed != 0 {
				og = restrOriginOfEntry(e)
			}
			return payDesc{class: pdSpell, card: e.card, src: NoSlot, og: og}
		case seAbility:
			return payDesc{class: pdAct, card: e.card, src: s.srcSlot()}
		}
		return payDesc{class: pdOther, src: NoSlot}
	}
	if exclude != NoObj {
		return payDesc{class: pdSpell, card: s.objs[exclude].card, src: NoSlot, og: s.restrOriginOfObj(exclude)}
	}
	return payDesc{class: pdOther, src: NoSlot}
}

// restrFloatAdmits is the mask of p's floating restricted batches (State.rb)
// the payment d describes may spend (gorge's planner pool for it is
// AvailableFor, which holds the batches RestrictValidMatches admits: a spell
// its restriction matches, an activated ability or a non-spell payment, CR
// 106.6). pdNone describes no payment: the batches count for nothing.
func (s *State) restrFloatAdmits(k *prep, p uint8, d *payDesc) uint8 {
	if d.class == pdNone || !s.rbAny(p) {
		return 0
	}
	switch d.class {
	case pdSpell:
		return s.restrSpendable(k, p, d.card, d.og)
	case pdAct:
		return s.restrPoolAct(k, p, d)
	}
	return s.restrPoolNonSpell(k, p)
}

// restrSrcAdmits reports whether restricted ability a of the source at sl
// may pay d (gorge pay.RestrictValidMatches; the chosen type is the mana
// source's own).
func (s *State) restrSrcAdmits(k *prep, p uint8, sl Slot, a *kop.Ability, d *payDesc) bool {
	if DebugRestrictedSource == 1 || DebugRestrictedSource == 2 {
		return DebugRestrictedSource == 1
	}
	switch d.class {
	case pdSpell:
		return s.restrSpellMatch(k, p, sl, a.ManaRestrict, d.card, d.og)
	case pdAct:
		if a.AFlags&kop.AfManaNonSpell != 0 {
			return true
		}
		if DebugManaRestrictActNone && a.ManaRestrictAb != 0 && restrActAnyCard(k, a.ManaRestrictAb) {
			return false
		}
		return a.ManaRestrictAb != 0 && s.restrActMatch(k, p, sl, a.ManaRestrictAb, d)
	}
	return a.AFlags&kop.AfManaNonSpell != 0
}

// restrPoolAct is the mask of p's restricted batches (State.rb) that may pay
// activated-ability payment d (gorge AvailableFor's RestrictValidMatches, CR
// 106.6).
//
//go:noinline
func (s *State) restrPoolAct(k *prep, p uint8, d *payDesc) uint8 {
	if k.gen == nil {
		return 0
	}
	var m uint8
	for i := range s.rb[SeatIx(p)] {
		b := &s.rb[SeatIx(p)][i]
		if b.kind == 0 {
			break
		}
		if b.units() != 0 && s.restrActB(k, p, b, d) {
			m |= 1 << i
		}
	}
	return m
}

// restrActB: batch b may pay activated-ability payment d. A batch names its
// restriction by genPrep.restrF index, not the source, so a chosen type is
// read from a source of p's with that restriction and the batch's choice, and
// a restriction with none admits as printed even if its source has left.
func (s *State) restrActB(k *prep, p uint8, b *rbatch, d *payDesc) bool {
	if s.restrActOnly(k, b.kind) {
		return true
	}
	for i := range k.t.Cards {
		abs := k.t.Cards[i].Abilities
		for j := range abs {
			a := &abs[j]
			if a.Kind != kop.AbMana || !k.gen.restrIsKind(b.kind, a) {
				continue
			}
			if a.AFlags&kop.AfManaNonSpell != 0 {
				return true
			}
			if a.ManaRestrictAb != 0 && k.t.Filters[a.ManaRestrictAb].Flags2&kop.F2ChosenType == 0 && s.restrActMatch(k, p, NoSlot, a.ManaRestrictAb, d) {
				return true
			}
		}
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		abs := k.t.Cards[s.cardAt(sl)].Abilities
		for j := range abs {
			if a := &abs[j]; a.Kind == kop.AbMana && k.gen.restrIsKind(b.kind, a) && a.ManaRestrictAb != 0 && s.perms[sl].chosen == b.chosen && s.restrActMatch(k, p, sl, a.ManaRestrictAb, d) {
				return true
			}
		}
	}
	return false
}

// restrActOnly: batch kind kind's restriction holds the activated-only marker
// (a resolving DB$ Mana's undotted RestrictValid$ Activated, kop.SpmActivated):
// it admits every activated-ability payment, from any source in any zone.
func (s *State) restrActOnly(k *prep, kind uint8) bool {
	for f := k.gen.restrFOf(kind); f != 0; f = k.t.Filters[f].Or {
		if k.t.Filters[f].Special&kop.SpmActivated != 0 {
			return true
		}
	}
	return false
}

// restrSpellMatch: card c, cast as a spell, matches the spell restriction f
// of the source at sl.
func (s *State) restrSpellMatch(k *prep, p uint8, sl Slot, f kop.FilterID, c CardID, og restrOrigin) bool {
	for ; f != 0; f = k.t.Filters[f].Or {
		fl := k.t.Filters[f]
		if fl.Special != 0 {
			continue // a Static.* alternative: only a special action's mana batch may spend it; the SpmActivated marker is refused here on purpose
		}
		fl.Zone = kop.ZoneNone // the spell is matched before it moves to the stack
		chosen := fl.Flags2&kop.F2ChosenType != 0
		fl.Flags2 &^= kop.F2ChosenType
		if !s.restrFilterMatch(k, &fl, c, p, og) {
			continue
		}
		if !chosen || s.restrChoseHas(k, sl, c) {
			return true
		}
	}
	return false
}

// restrChoseHas: card c has the creature type the source at sl chose.
func (s *State) restrChoseHas(k *prep, sl Slot, c CardID) bool {
	ch, ok := s.chosenTypeSub(sl)
	if !ok {
		return false
	}
	cd := &k.t.Cards[c]
	return int(ch) < len(k.t.Subtypes) && cd.Subtypes.Has(int(ch)) || cd.AllCreatureTypes
}

// restrActMatch: the activated ability d describes has a source matching
// the activated restriction f of the mana source at sl.
func (s *State) restrActMatch(k *prep, p uint8, sl Slot, f kop.FilterID, d *payDesc) bool {
	if d.src != NoSlot {
		return s.matchPerm(k, f, d.src, p, sl)
	}
	for ; f != 0; f = k.t.Filters[f].Or {
		fl := k.t.Filters[f]
		if fl.Special != 0 {
			continue // a Static.* alternative: never an activated ability
		}
		fl.Zone = kop.ZoneNone
		chosen := fl.Flags2&kop.F2ChosenType != 0
		fl.Flags2 &^= kop.F2ChosenType
		if s.matchCardOne(k, &fl, d.card, p, p, NoObj) && (!chosen || s.restrChoseHas(k, sl, d.card)) {
			return true
		}
	}
	return false
}

// restrUsable: the permanent at sl can be tapped for mana now (its own
// mana ability aside: canTapForManaPC).
func (s *State) restrUsable(k *prep, sl Slot) bool {
	pm := &s.perms[sl]
	return pm.flags&pfTapped == 0 && pm.ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0 &&
		!(s.activationSick(k, sl)) && !s.genLost(k, sl) &&
		!(k.gen.anyCantAct && s.actForbidden(k, sl, true)) // a CantBeActivated lock (cast_cantactivate.go)
}

// restrView is the planner's view of the restricted ability of the source
// at sl for payment d (nil: none, or d may not spend it): own is the
// source's view of its census ability. kind is the record's, ab the
// ability. A widened source whose restricted ability d may not spend
// (widen: true, v nil) is still a unit of its census ability, but ranks
// as the source it is: gorge's Alt.Flex and Alt.Colours describe the
// SOURCE over every normal alternative, withheld or not (pay/planalts.go).
func (s *State) restrView(k *prep, p uint8, sl Slot, own *pcard, d *payDesc) (v *pcard, kind uint8, ab *kop.Ability) {
	v, kind, ab, _, _ = s.restrViewW(k, p, sl, own, d)
	return v, kind, ab
}

// restrViewW is restrView, also reporting the colours a widened source's
// unit ranks over when d may not spend the restricted ability (dmask) and the
// index of the admitting record's ability (abi: -1 with no view), which a card
// of several restricted abilities cannot recover from its first record.
func (s *State) restrViewW(k *prep, p uint8, sl Slot, own *pcard, d *payDesc) (v *pcard, kind uint8, ab *kop.Ability, dmask uint8, abi int8) {
	g := k.gen
	c := s.cardAt(sl)
	// a card may hold several restricted abilities (Steelswarm Operator):
	// the first whose half admits the payment is the source's unit. A
	// widened record the payment may not spend still ranks as the source it
	// is (dmask), so it is remembered when no record admits.
	for i := range g.rs.recs {
		rec := &g.rs.recs[i]
		if rec.card != c {
			continue
		}
		a := &k.t.Cards[c].Abilities[rec.ab]
		switch rec.kind {
		case rkWiden:
			if own.manaAb < 0 || !s.canTapForManaPC(k, sl, own) {
				continue
			}
			if !s.restrSrcAdmits(k, p, sl, a, d) {
				if dmask == 0 {
					dmask = srcMask(&g.rs.views[rec.view])
				}
				continue
			}
		default:
			if !s.restrUsable(k, sl) || !s.restrSrcAdmits(k, p, sl, a, d) {
				continue
			}
		}
		view := rec.view
		if rec.kind == rkDyn {
			n := s.dynManaN(k, a, sl)
			if n <= 0 {
				continue
			}
			view += int32(min(n, maxDynMana))
		}
		return &g.rs.views[view], rec.kind, a, 0, rec.ab
	}
	return nil, 0, nil, dmask, -1
}

// restrOnlyUsable: the source at sl has no census mana ability and a
// restricted one payment d may spend (a planner unit of its own).
func (s *State) restrOnlyUsable(k *prep, p uint8, sl Slot, d *payDesc) bool {
	c := s.cardAt(sl)
	if k.gen.flags[c]&gfRestrSrc == 0 || k.cards[c].manaAb >= 0 {
		return false
	}
	v, _, _ := s.restrView(k, p, sl, &k.cards[c], d)
	return v != nil
}

// restrSrcAffords: av, with the restricted sources payment d may spend
// counted, pays cost plus extra generic. skip is a source the payment's own
// {T} taps. Every caller asks only when av alone does not pay, so a
// payment no restricted source of the table can widen enough is false
// without the walk (restrSrcNone).
func (s *State) restrSrcAffords(k *prep, p uint8, d payDesc, cost *kop.ManaCost, av *avail, skip Slot, extra int) bool {
	if restrSrcNone(k.gen.rs, &d, cost, av, extra) {
		return false
	}
	t, multi, nm := s.restrSrcAvailD(k, p, d, av, skip, false)
	if t.affordsMulti(cost, extra, multi[:nm]) {
		return true
	}
	if s.deferLive(k) && s.g.availX&(availWalk|availHyp) == 0 {
		// Only a Taps-deferred restricted source pays it (Hydraulic Helper's
		// artifact-spells {U} funding Super Suit's equip, Steelswarm Operator
		// under Cryoshatter): gorge's census leaves the source out and its
		// prefix search pays the play by tapping the source first, its
		// watcher's trigger stacked beneath and resolved before even a
		// sorcery-speed play (CR 601.2g, 602.5d; the script route models the
		// one-fixed-colour shape, mana_tapsdefer.go restrTapsGap). A source
		// with a choice between restricted abilities (Steelswarm Operator's
		// activation-only {U}{U}) the route models no ability choice of:
		// an instant-speed play still stalls (the declared limit), while a
		// sorcery-speed one keeps the old behaviour and is not offered.
		if t, multi, nm = s.restrSrcAvailD(k, p, d, av, skip, true); t.affordsMulti(cost, extra, multi[:nm]) &&
			!s.restrDeferScriptable(k, p, skip) && d.instant {
			s.capFull(StallRestrDefer)
		}
	}
	return false
}

// restrDeferScriptable: every restricted source gorge's planner defers
// (restrSrcAvailD's withDeferred arm newly counts) is one the script route
// funds -- a member of scriptTapsDefer's mask, the single-restricted-ability
// Taps-deferred shape -- so restrSrcAffords need not stall the payment. A
// deferred source that cannot be activated now (tapped, summoning-sick,
// locked) funds nothing and is no reason to stall.
func (s *State) restrDeferScriptable(k *prep, p uint8, skip Slot) bool {
	var mask slotMask
	have := false
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if sl == skip || k.gen.flags[s.cardAt(sl)]&gfRestrSrc == 0 || !s.planDeferred(k, p, sl) {
			continue
		}
		if !have {
			mask, have = s.scriptTapsDefer(k, p), true
		}
		if !mask.has(sl) && s.tapsGapActivatable(k, sl) {
			return false
		}
	}
	return true
}

// restrSrcNone: no restricted source of the table can make av pay cost
// plus extra for d. PRECONDITION (unchecked, for speed): av alone does not
// pay cost plus extra -- the class shortcut answers "no help", not "cannot
// pay", so a caller holding an av that may pay must test av.affords first.
// Each restrSrcAffords call site says so where it is reached: a non-spell payment on a table whose restrictions all
// name spells (restrSrcAdmits never admits it), or a widen-only table (the
// sources' units are already av's, their colours widened: the mana total
// stays av's) short of the cost's mana total.
func restrSrcNone(rs *restrData, d *payDesc, cost *kop.ManaCost, av *avail, extra int) bool {
	if d.class != pdSpell && !rs.nonSpell && DebugRestrictedSource != 1 {
		return true
	}
	if rs.ext || av.pain.on {
		return false
	}
	tot := int(cost.Generic) + extra
	for _, n := range cost.Pips {
		tot += int(n)
	}
	return tot > av.total
}

// restrSrcAvail is a copy of av with the restricted sources payment d may
// spend counted: a widened source's own unit becomes a unit of the widened
// colours; an evaluated amount (Helga) is returned apart (multi[:nm]): that
// many mana of one colour.
func (s *State) restrSrcAvail(k *prep, p uint8, d payDesc, av *avail, skip Slot) (t avail, multi [2]uint8, nm int) {
	return s.restrSrcAvailD(k, p, d, av, skip, false)
}

// restrSrcAvailD is restrSrcAvail; withDeferred counts the Taps-deferred
// sources too. Without it they follow availFor's census gate: gorge's census
// never lists a source whose tap fires an unvetted Taps watcher
// (mana_tapsdefer.go), restricted or not, so its restricted mana funds no
// planned payment; the potential walks still count it.
func (s *State) restrSrcAvailD(k *prep, p uint8, d payDesc, av *avail, skip Slot, withDeferred bool) (t avail, multi [2]uint8, nm int) {
	t = *av
	deferSkip := !withDeferred && s.deferLive(k) && s.g.availX&(availWalk|availHyp) == 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if sl == skip || k.gen.flags[s.cardAt(sl)]&gfRestrSrc == 0 {
			continue
		}
		if deferSkip && s.planDeferred(k, p, sl) {
			continue
		}
		own := s.manaPCGen(k, sl)
		v, kind, _ := s.restrView(k, p, sl, own, &d)
		if v == nil {
			continue
		}
		switch kind {
		case rkWiden:
			if d.noPair && v.wideChosen {
				break // the census alone: the pair is no unit of it
			}
			t.remove(s.srcMaskAt(own, sl), own.manaN)
			if v.wideN != 0 && DebugRestrictedSource != 4 {
				// the larger alternative holds everything the census one
				// makes (its colours include them), so the source counts as
				// wideN mana of one colour of its choosing
				mb := s.restrWideMask(v, sl)
				t.addUnits(mb, v.wideN)
				if !v.wideChosen && v.wideCol == kop.ColAny {
					s.addOneColour(&t, v.wideN) // a fixed colour is no free choice
				}
				break
			}
			t.addUnits(s.srcMaskAt(v, sl), v.manaN)
			t.addXv(v)
		case rkExt:
			t.addUnits(s.srcMaskAt(v, sl), v.manaN)
			t.addXv(v)
		case rkDyn:
			if nm == len(multi) {
				s.capFull(StallRestrSrc)
				continue
			}
			multi[nm] = v.manaN
			nm++
		}
	}
	return t, multi, nm
}

// restrWideMask is the colour mask of a widened source's larger alternative:
// wideCol, or for a chosen-colour one the permanent's chosen colour (and the
// ability's printed one) at sl (Sunken Citadel).
func (s *State) restrWideMask(v *pcard, sl Slot) uint8 {
	if v.wideChosen {
		return s.srcMaskAt(v, sl)
	}
	return uint8(v.wideCol)
}

// addXv adds an all-at-once source's fixed rest (pcard.manaXv) to a: a
// restricted ManaEach ability's extra colours are restricted too (Tin Street
// Gossip's {R}{G}).
func (a *avail) addXv(v *pcard) {
	for c, n := range v.manaXv {
		if n > 0 {
			a.addUnits(1<<c, n)
		}
	}
}

// affordsMulti: a, with each of ns a unit of that many mana of one colour
// (any of W U B R G), pays cost plus extra generic.
func (a avail) affordsMulti(cost *kop.ManaCost, extra int, ns []uint8) bool {
	if len(ns) == 0 {
		return a.affords(cost, pipMaskOf(cost), extra)
	}
	for c := 0; c < 5; c++ {
		b := a
		b.addUnits(1<<c, ns[0])
		if b.affordsMulti(cost, extra, ns[1:]) {
			return true
		}
	}
	return false
}

// restrFallback: the hand cast of card c for p, which av does not pay,
// still pays through p's restricted mana (State.rb) or the restricted sources
// its spell payment may spend.
//
//go:noinline
func (s *State) restrFallback(k *prep, p uint8, c CardID, og restrOrigin, cost *kop.ManaCost, av *avail) bool {
	if s.rbAny(p) {
		// the pool beside the restricted sources (gorge AvailableFor holds both)
		t := *av
		if s.restrAvail(k, p, c, og, &t) {
			if t.affords(cost, pipMaskOf(cost), 0) {
				return true
			}
			av = &t
		}
	}
	// restrSrcNone precondition: every caller tests !av.affords first.
	return k.restrSrc && s.restrSrcAffords(k, p, payDesc{class: pdSpell, card: c, src: NoSlot, og: og, instant: k.cards[c].instant || s.castFlash(k, p, c)}, cost, av, NoSlot, 0)
}

// payRiderRestr is payRider on a table with restricted planner sources: a
// restricted evaluated amount (Helga) the plan taps floats restricted
// (State.rb), the payment spends it before the ordinary mana of its colour
// (gorge emitRestrictedManaSpend carves the matching batches first) and
// what it leaves stays restricted (gorge's RestrictedMana).
//
//go:noinline
func (s *State) payRiderRestr(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, anyColor bool) {
	sv := s.rb[SeatIx(p)]
	var pk planner
	s.tapForPK(k, p, cost, x, exclude, false, anyColor, &pk)
	if !pk.rbOn && !pk.rdynOn {
		s.payFromPoolRider(k, p, cost, x, anyColor)
		return
	}
	pl := &s.pl[p]
	var tk [maxRB][kop.NumManaKinds]uint8
	ord := pl.pool
	s.rbTakeNew(p, &sv, &tk)
	pre := pl.pool
	s.payFromPoolRider(k, p, cost, x, anyColor)
	s.rbSettle(p, &tk, &ord, &pre, pk.rbOn && s.rbTyped(p))
}

// filtRestrMask: the colours of the restricted ability of the source at sl
// a script's funding step may activate now (0: none; an all-at-once
// ability's lowest colour alone, its one answer). A widened source
// needs its census ability usable too (gorge lists the source's activation
// once; the ask offers both abilities).
func (s *State) filtRestrMask(k *prep, sl Slot) uint8 {
	c := s.cardAt(sl)
	if k.gen.flags[c]&gfRestrSrc == 0 {
		return 0
	}
	rec := k.gen.restrRecOf(c)
	if rec == nil || rec.kind == rkDyn || !s.restrUsable(k, sl) {
		return 0
	}
	ab := &k.t.Cards[c].Abilities[rec.ab]
	m := uint8(ab.ManaColors) & 0x3f
	if ab.ManaEach {
		// an all-at-once ability (Troyan's {G}{U}) is one activation with no
		// colour ask in gorge, and floatRestrAb floats every colour: a single
		// answer, so the search charges and branches as gorge does (CR 106.1)
		m &= -m
	}
	return m
}

// filtRestrDo plays the restricted answer st: the source's restricted mana
// floats (State.rb).
func (s *State) filtRestrDo(k *prep, p uint8, st filtStep) {
	c := s.cardAt(st.sl)
	rec := k.gen.restrRecOf(c)
	ab := &k.t.Cards[c].Abilities[rec.ab]
	if !s.floatRestrAb(k, p, st.sl, ab, int(st.ans), max(ab.ManaN, 1)) {
		s.capFull(StallRestrSrc) // every batch is taken: never a silent spill
	}
}

// filtDeadEnd: the script fs found leaves a play the walk no longer lists: the
// planner reads the pool through the restriction (pay.PlanPool), so the oracle
// plays the script only when the play stays offered once it has run (gorge.go
// scriptCompletes).
func (s *State) filtDeadEnd(k *prep, p uint8, a Action, fs *filtSearch) bool {
	restr, passed, sp0 := false, false, s.sp
	for i := 0; i < fs.n; i++ {
		restr = restr || fs.path[i].restr
		switch fs.path[i].other.kind {
		case otherAnswer, otherPass:
			passed = true
		}
	}
	t := s.fork()
	defer s.unfork()
	t.g.noGrant = a.Kind() == AkActivate // the activation's granted mana is the grant search's
	t.filtPlay(k, p, fs.path[:fs.n])
	if !passed {
		passed = t.npend != 0 || t.sp != sp0 // a step the search settled through (try's npend grew)
	}
	if passed {
		// The search found the play at the node its settle reached: after the
		// replay's last pass or answer (the Pizza's sacrifice triggering Super
		// Shredder's leaves-the-battlefield counter) the position is answered
		// with the trigger still on the stack, where the sorcery gate drops
		// the play. Settle to p's next decision, as the replay of every other
		// search's path does (tapXPlay's trailing tapXNext, grantApply's).
		var settled Decision
		if !t.tapXNext(p, &settled) {
			return true // the replay cannot settle: the play is not offered
		}
	}
	var d Decision
	if !restr {
		t.restrOptions(k, &d, p)
		return !hasOption(&d, a) // priced by mana alone, its own source tapped by the script
	}
	return !t.scriptCompletes(k, p, a)
}

// slotNoneUnkicked: target slot t takes no object unless the spell was
// kicked (Tear Asunder's "if this spell was kicked, exile target nonland
// permanent instead", announced on cast): a kicked
// range with no unkicked bounds and a kicked minimum. The default of one
// object applies to every other slot with Min|Max == 0.
func slotNoneUnkicked(t *kop.TargetSpec) bool {
	return t.TFlags&kop.TgfKickedRange != 0 && t.Min|t.Max == 0 && t.MinKicked > 0
}

// castAffordsR: av pays the cast of card c at cost, or p's restricted
// mana floating (State.rb, a spell it admits sees it in its pool: restrAffords)
// or the restricted sources the cast may spend make it payable.
func (s *State) castAffordsR(k *prep, p uint8, c CardID, og restrOrigin, cost *kop.ManaCost, av *avail) bool {
	if av.affords(cost, pipMaskOf(cost), 0) {
		return true
	}
	if s.rbAny(p & 1) {
		t := *av
		if s.restrAvail(k, p, c, og, &t) {
			if t.affords(cost, pipMaskOf(cost), 0) {
				return true
			}
			av = &t
		}
	}
	// restrSrcNone precondition: restrSrcAffords runs only once av.affords failed (above).
	return k.restrSrc && s.restrSrcAffords(k, p, payDesc{class: pdSpell, card: c, src: NoSlot, og: og, instant: k.cards[c].instant || s.castFlash(k, p, c)}, cost, av, NoSlot, 0)
}

// restrFaceAffords: a hybrid pip's face r (plus extra generic) is payable once
// the restricted sources the payment being built may spend are counted (CR
// 106.6): gorge's cast window prices a RestrictValid$ ability its payment
// admits like any other, so the face's colour can come from it. av does not
// pay r alone (restrSrcNone's precondition).
//
//go:noinline
func (s *State) restrFaceAffords(k *prep, p uint8, r *kop.ManaCost, av *avail, extra int) bool {
	d := s.payDescNow(NoObj)
	skip := NoSlot
	if d.class == pdAct {
		skip = d.src
	}
	return s.restrSrcAffords(k, p, d, r, av, skip, extra)
}

// wideFaceAffords: a hybrid pip's face r (plus extra generic) is payable once
// the wide any-colour restricted sources the payment being built may spend are
// counted (CR 601.2b, 601.2g, 106.6): Flamebraider's two mana of any colours
// pay an Elemental's pip in either colour. gorge's offer gate reaches the
// face through the cast window, which activates such a source like any other;
// restrFaceAffords does not see it (its census mana is a gap, not a planner
// unit). av does not pay r alone.
//
//go:noinline
func (s *State) wideFaceAffords(k *prep, p uint8, r *kop.ManaCost, av *avail, extra int) bool {
	d := s.payDescNow(NoObj)
	skip := NoSlot
	if d.class == pdAct {
		skip = d.src
	}
	t := *av
	any := false
	for b := s.scriptWide(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if sl == skip {
			continue
		}
		if s.restrSrcAdmits(k, p, sl, &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.anyR(s.cardAt(sl))], &d) {
			t.addUnits(uint8(kop.ColAny), 2)
			any = true
		}
	}
	return any && t.affords(r, pipMaskOf(r), extra)
}

// restrSrcMaxX is the largest X the payment being built can announce with
// p's restricted sources it may spend counted (-1: none) -- the X ask's
// bound (cast.go bsX), av less the activation's own tapped source. A wide
// any-colour restricted source (Web Watch) is a scripted census gap, not an
// rk record, so it is counted here for the payment it admits (CR 106.6).
//
//go:noinline
func (s *State) restrSrcMaxX(k *prep, p uint8, cost *kop.ManaCost, av *avail) int {
	d := s.payDescNow(NoObj)
	skip := NoSlot
	if d.class == pdAct {
		skip = d.src
	}
	t, multi, nm := s.restrSrcAvail(k, p, d, av, skip)
	s.wideSrcAvailAdd(k, p, &t, &d, skip)
	return t.maxXMulti(cost, pipMaskOf(cost), multi[:nm])
}

// wideSrcAvailAdd adds p's wide any-colour restricted sources (Web Watch's
// Combo Any x2, restrWideAnyMana) that may spend on payment d: each makes
// two mana of any combination of colours (CR 106.6, 601.2b), so the X bound
// reads it as two independent any-colour units.
func (s *State) wideSrcAvailAdd(k *prep, p uint8, a *avail, d *payDesc, skip Slot) {
	if k.gen == nil || !k.gen.anyMultiR {
		return
	}
	for b := s.scriptWide(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if sl == skip {
			continue
		}
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.anyR(s.cardAt(sl))]
		if s.restrSrcAdmits(k, p, sl, ab, d) {
			a.addUnits(uint8(kop.ColAny), 2)
		}
	}
}

// maxXMulti is maxX with each of ns a unit of that many mana of one colour.
func (a avail) maxXMulti(cost *kop.ManaCost, pipMask uint8, ns []uint8) int {
	if len(ns) == 0 {
		return a.maxX(cost, pipMask)
	}
	best := -1
	for c := 0; c < 5; c++ {
		b := a
		b.addUnits(1<<c, ns[0])
		best = max(best, b.maxXMulti(cost, pipMask, ns[1:]))
	}
	return best
}

// actRestrAffords: av, with the restricted sources an activation of the
// ability of the source at sl may spend counted, pays cost. skip is the
// source the activation's own {T} taps. Callers ask only after av alone
// failed (legal.go abilityAffords): restrSrcNone's precondition.
//
//go:noinline
func (s *State) actRestrAffords(k *prep, sl Slot, a *kop.Ability, cost *kop.ManaCost, av *avail, skip Slot) bool {
	p := s.perms[sl].ctrl
	d := payDesc{class: pdAct, card: s.cardAt(sl), src: sl, instant: a.Cost.Flags&kop.CostSorcery == 0}
	if m := s.restrPoolAct(k, p, &d); m != 0 {
		// the floating restricted mana beside the restricted sources
		// (gorge AvailableFor holds both)
		t := *av
		s.restrAvailAdd(p, m, &t)
		if t.affords(cost, pipMaskOf(cost), 0) {
			return true
		}
		av = &t
	}
	return s.restrSrcAffords(k, p, d, cost, av, skip, 0)
}

// actPoolAffords: av, with p's floating restricted mana counted, pays the
// activated ability of the source at sl, which may spend it.
//
//go:noinline
func (s *State) actPoolAffords(k *prep, sl Slot, cost *kop.ManaCost, av *avail) bool {
	p := s.perms[sl].ctrl
	d := payDesc{class: pdAct, card: s.cardAt(sl), src: sl}
	return s.actPoolAffordsDesc(k, p, cost, d, av)
}

// actPoolAffordsDesc: av, with the floating restricted batches d admits
// counted, pays cost: the same test for an activation whose source is not on
// the battlefield (a graveyard card, src NoSlot), CR 106.6.
//
//go:noinline
func (s *State) actPoolAffordsDesc(k *prep, p uint8, cost *kop.ManaCost, d payDesc, av *avail) bool {
	m := s.restrPoolAct(k, p, &d)
	if m == 0 {
		return false
	}
	t := *av
	s.restrAvailAdd(p, m, &t)
	return t.affords(cost, pipMaskOf(cost), 0)
}

// poolPaysAct reports whether p's floating pool alone pays cost for the
// activation d describes: gorge prices an {X} activation against the pool it
// would spend (AvailableFor), the restricted batches the activation may spend
// (CR 106.6) included. Cold, out of line.
//
//go:noinline
func (s *State) poolPaysAct(k *prep, p uint8, cost *kop.ManaCost, d payDesc) bool {
	if s.poolPays(p, cost) {
		return true
	}
	m := uint8(0)
	if s.rbAny(p) {
		m = s.restrPoolAct(k, p&1, &d)
	}
	if m == 0 {
		return false
	}
	var av avail
	for c := 0; c < kop.NumManaKinds; c++ {
		if n := s.pl[SeatIx(p)].pool[c]; n > 0 {
			av.addUnits(1<<c, n)
		}
	}
	s.restrAvailAdd(p&1, m, &av)
	return av.affords(cost, pipMaskOf(cost), 0)
}

// Restricted sources whose alternatives make different amounts (gorge
// pay/planalts.go appendUnitAlternatives): a source with a census ability
// of one mana and a restricted Produced$ Any ability of a literal amount
// above one (Intrepid Stablemaster: {G}, or two mana of one colour that only
// a Mount or Vehicle spell may spend) is one unit whose alternatives are the
// census one and one of the larger amount per colour. A payment that admits
// the restriction plans it as a pair of unit types sharing the source (the
// painland's pair of mana_pain.go): the census alternative and the larger
// one, which ranks as the source it is (flexibility and colours over every
// alternative) and whose taps float restricted, so the plan's surplus (rank
// key 4) is restricted mana left over (CR 106.6; a payment spends the
// restricted mana of a colour before the ordinary one, gorge
// emitRestrictedManaSpend). The witness hands a class's chosen units the
// census alternative first (pay.CompareSteps: ability index).

// restrPairUnit registers the widened source sl (view v, whose wideN is
// set) at pcs index i as a pair of unit types. It reports whether it
// handled the source (a source the planner has no room for stalls loud).
// restricted is false for a gated pair (Whisperer of the Wilds, mana_gate.go),
// whose larger alternative makes ordinary mana: the same unit, no restricted
// float.
//
// recAb is the restricted ability whose view v is (restrViewW's abi); negative
// (a gated pair) looks the card's record up.
//
//go:noinline
func (s *State) restrPairUnit(k *prep, sl Slot, v *pcard, pk *planner, i int, restricted bool, recAb int8) bool {
	own := s.manaPCGen(k, sl)
	ma, na := s.srcMaskAt(own, sl), own.manaN
	mb, nb := s.restrWideMask(v, sl), v.wideN
	if ma&^mb != 0 || own.manaAb < 0 || restricted && DebugRestrictedSource == 4 {
		return false // not the shape kgen lowers: the ordinary widened unit
	}
	cr := s.isCreature(k, sl)
	dmk := srcMask(v) // the colours both alternatives rank over
	if v.wideChosen {
		// both alternatives make the permanent's chosen colour (and any
		// printed one), read at this slot: the pair type ranks over it
		dmk = mb
	}
	var xv [kop.NumManaKinds]uint8
	var xm, xt uint8
	if k.gen != nil && k.gen.anyTapExtra {
		xv = s.srcExtra(k, sl, own) // the same fixed extra on either alternative
		for c, n := range xv {
			if n > 0 {
				xm |= 1 << c
				xt += n
			}
		}
	}
	ta, tb := -1, -1
	for t := 0; t < pk.nt && ta < 0; t++ {
		a := &pk.types[t]
		if a.pair == 0 || a.alt || a.rb || a.pain != 0 || a.mask != ma || a.n != na || a.dmask != dmk || a.creat != cr || a.xv != xv {
			continue
		}
		if b := &pk.types[a.pair-1]; b.rb == restricted && b.mask == mb && b.n == nb {
			ta, tb = t, int(a.pair)-1
		}
	}
	if ta < 0 {
		if pk.nt+2 > maxTypes || pk.nu >= maxUnits {
			s.capFull(StallRestrSrc)
			return true
		}
		fl := uint8(bits.OnesCount8(dmk | xm))
		ta, tb = pk.nt, pk.nt+1
		pk.nt += 2
		pk.types[ta] = utype{mask: ma, dmask: dmk, n: na, creat: cr, flex: fl, pair: uint8(tb) + 1, xv: xv, xmask: xm, xt: xt}
		pk.types[tb] = utype{mask: mb, dmask: dmk, n: nb, creat: cr, flex: fl, pair: uint8(ta) + 1, alt: true, rb: restricted, xv: xv, xmask: xm, xt: xt}
		pk.maxXt = pk.maxXt || xt != 0
		pk.maxN = max(pk.maxN, int(max(na, nb))+int(xt))
		pk.pairs = true
	}
	pk.types[ta].avail++
	pk.types[tb].avail++
	pk.units[pk.nu] = sl
	if pk.bfl {
		pk.uobj[pk.nu] = Obj(i)
	} else {
		pk.uobj[pk.nu] = s.perms[sl].obj
	}
	pk.utyp[pk.nu] = uint8(ta)
	pk.rkind[pk.nu] = 0
	pk.rAb[pk.nu] = v.manaAb
	if recAb >= 0 {
		pk.rAb[pk.nu] = recAb // the pair's restricted alternative
	} else if k.gen.rs != nil {
		if rec := k.gen.restrRecOf(s.cardAt(sl)); rec != nil {
			pk.rAb[pk.nu] = rec.ab
		}
	}
	pk.nu++
	for c := 0; c < 5; c++ {
		if (dmk|xm)&(1<<c) != 0 {
			pk.colSrc[c]++
		}
	}
	return true
}
