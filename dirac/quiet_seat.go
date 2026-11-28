package dirac

import (
	"fmt"
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The quiet-seat proof (docs/design/dirac-quiet-seat-proof.md): an
// over-approximating proof that the seat holding priority can only pass,
// which never computes an option. Most posed priority walks in a four-seat
// pod are a seat at instant speed with nothing to do; the walk still runs
// every section, the script walk's nested walks included. seatQuiet shows
// instead that every source of a non-pass option in priorityOptions is ruled
// out by a cheap necessary condition (timing, a zone the card cannot act
// from, a tapped {T} cost, a mana floor above every mana the seat could
// make); anything it cannot rule out leaves the walk to run exactly as
// before. gorge's engine carries the same proof (engine/rules/quiet_facts.go,
// docs/design/2026-10-08-quiet-seat-walk-skip-design.md).
//
// Every rule below is a necessary condition of one walk route at instant
// speed (the proof refuses a sorcery-speed window outright), listed in the
// design note's obligation table. DebugQuietVerify runs the walk beside the
// proof and counts a proved window whose walk offers anything but pass, or
// whose walk changes the State.

// quietFact is one card's quiet facts (prep.quietCard), computed once per
// table. A floor is the least mana a payment can take (quietFloor);
// quietNone is no such ability, quietInf an unbounded amount.
type quietFact struct {
	bits uint16
	cast uint8 // the cast's mana floor (meaningless with qOpen)
	// bfAny, bfTap: the least floor over the card's own battlefield activated
	// abilities that are not sorcery-speed, without and with {T}
	bfAny, bfTap uint8
	hand         uint8 // the least floor over its hand-zone activated abilities that are not sorcery-speed
	// mtap: the most mana one {T} of a printed mana ability makes; mfree:
	// the mana its printed mana abilities without {T} make, together
	mtap, mfree uint8
	// grAny, grTap: the least floor over the card's latent activated
	// abilities that are not sorcery-speed (the bodies a grant from this
	// card hands another permanent), without and with {T}
	grAny, grTap uint8
}

const (
	quietNone uint8 = 255
	quietInf  uint8 = 255
)

// quietFact.bits.
const (
	// qInst: the card, or one of its faces, is castable at instant speed by
	// its own text (pcard.instant: instant type or flash).
	qInst uint16 = 1 << iota
	// qHand: from the hand the card has a route the hand loop offers ahead
	// of (or beside) the plain cast's timing gate: a face or face-down cast,
	// foretell, sneak, offering, may-flash or a teamwork self grant.
	qHand
	// qGrave: from its owner's graveyard: a graveyard activated ability that
	// is not sorcery-speed, its own graveyard may-play static, or a printed
	// escape, mayhem or retrace cast (walks the proof does not read further).
	qGrave
	// qExile: from exile: an exile activated ability that is not
	// sorcery-speed, or its own exile may-play static.
	qExile
	// qGyCast: a printed graveyard cast route that keeps the card's own
	// timing: flashback, aftermath, disturb, harmonize, beam me up, a
	// graveyard warp, or its own graveyard alternative-cost static.
	qGyCast
	// qOpen: the cast's price is not bounded below by its mana cost: an
	// alternative or credited cost (convoke, delve, evoke, ...), a self cost
	// static, a Phyrexian or no mana cost.
	qOpen
	// qMod: the card prints a static the proof does not read that may lower
	// another payment (a cost reduction, an alternative cost, a stack keyword
	// grant): while one is on the battlefield every floor reads 0.
	qMod
	// qMana: on the battlefield the card adds mana the bound does not count
	// per permanent: a tap-extra static (every bound is then unbounded).
	qMana
	// qGMana: on the battlefield the card may grant a mana ability: every
	// untapped permanent counts the largest granted amount.
	qGMana
	// qPlain: the card's hand cast has one route: no kicker, multikicker,
	// offspring, buyback, entwine, teamwork, casualty, bargain, conspire,
	// optional, bestow, mutate or either-or extra cost.
	qPlain
)

// quietHarmless is the static kinds that cannot lower a payment, add mana
// the bound does not count, or add a route the proof does not read (the
// flash, may-play, equip-flash and loyalty-flash kinds are read on their
// own): every other kind, a kind added later included, is qMod.
func quietHarmless(sk kop.StaticKind) bool {
	switch sk {
	case kop.StatReduceCost, kop.StatAlternativeCost, kop.StatSetCost, kop.StatRetraceGrant, kop.StatCyclingAlt,
		kop.StatCyclingGrant, kop.StatDefiler, kop.StatMadnessGrant, kop.StatMiracleGrant, kop.StatEscapeGrant,
		kop.StatReduceAbility, kop.StatWarpGrant, kop.StatFbGrant, kop.StatStackKw, kop.StatBeamGrant, kop.StatTapExtra,
		kop.StatTapPower:
		return false
	}
	return sk < kop.StatCount || sk >= kop.StatOptionalAttackCost && sk <= kop.StatUntapAdjust ||
		sk >= kop.StatEscapeGrant && sk <= kop.StatLethalByPower
}

// quietFloor is the least mana that pays m: X at 0, a Phyrexian pip paid
// with life, a hybrid or two-brid pip one mana.
func quietFloor(m *kop.ManaCost) uint8 {
	n := int(m.Generic) + int(m.Snow)
	for _, p := range m.Pips {
		n += int(p)
	}
	switch {
	case m.Phyrexian():
	case m.Hyb[0] == kop.HybTwobrid:
		n += m.TbN()
	default:
		for _, h := range m.Hyb {
			if h != 0 {
				n++
			}
		}
	}
	return uint8(min(n, 254))
}

// quietAbFloor is activated ability a's floor: 0 when a cost credit or
// reduction of its own can lower it.
func quietAbFloor(a *kop.Ability) uint8 {
	if a.OwnReduce != (kop.ValueRef{}) || a.Cost.Flags&(kop.CostPowerUp|kop.CostWaterbend) != 0 {
		return 0
	}
	return quietFloor(&a.Cost.Mana)
}

// quietManaAmt is the mana one activation of mana ability a makes, quietInf
// when the amount is not fixed.
func quietManaAmt(a *kop.Ability) int {
	if a.AFlags&kop.AfManaDynamic != 0 {
		return int(quietInf)
	}
	if a.AFlags&kop.AfManaEachColor != 0 {
		return 5 // one of each colour among some permanents
	}
	n := max(int(a.ManaN), 1)
	if a.ManaEach {
		c := bits.OnesCount8(uint8(a.ManaColors))
		if a.ManaColors&kop.ColAny == kop.ColAny {
			c = 6
		}
		n *= max(c, 1)
	}
	return n
}

// prepQuiet computes the table's quiet facts. quietOff is set when the table
// carries a route the proof does not model at all: every seat runs the walk.
func prepQuiet(t *kop.Table, k *prep) {
	k.quietCard = make([]quietFact, len(t.Cards))
	k.quietGrantAny, k.quietGrantTap, k.quietGMTap = quietNone, quietNone, 0
	g := k.gen
	if g != nil && (g.anyAnyAct || g.anyExileGrant || g.uses.has(useGainsOf) || g.anyMutate || g.anyMerge ||
		g.anyCondSplit || g.anyEscapeGrant || tableHasStatic(t, kop.StatRetraceGrant)) || k.anyCyclingGrant {
		k.quietOff = true
	}
	equipFlash := g != nil && g.uses.has(useEquipFlash)
	loyFlash := tableHasStatic(t, kop.StatLoyaltyFlash)
	sorcOnly := func(a *kop.Ability) bool {
		f := a.Cost.Flags
		return f&kop.CostSorcery != 0 && !(equipFlash && a.AFlags&kop.AfEquip != 0) || f&kop.CostLoyalty != 0 && !loyFlash
	}
	for i := range t.Cards {
		cd := &t.Cards[i]
		pc := &k.cards[i]
		qf := quietFact{bfAny: quietNone, bfTap: quietNone, hand: quietNone, grAny: quietNone, grTap: quietNone}
		q := uint16(0)
		if pc.instant || pc.kw&kop.KwFlash != 0 {
			q |= qInst
		}
		for _, f := range cd.Faces {
			if int(f) < len(k.cards) && (k.cards[f].instant || k.cards[f].kw&kop.KwFlash != 0) {
				q |= qInst
			}
		}
		if len(cd.Faces) != 0 || cd.FaceMode != 0 || cd.MorphFamily != 0 || cd.HasForetell || cd.HasSneak ||
			cd.HasOffering || cd.HasMayFlash || cd.SelfFlashSA == kop.SfsaTeamwork {
			q |= qHand
		}
		if cd.Escape || cd.Mayhem || cd.Retrace {
			q |= qGrave
		}
		if pc.kw&kop.KwFlashback != 0 || cd.FlipCast == kop.FlipDisturb || cd.Harmonize || cd.BeamMeUp || cd.HasWarp && cd.WarpGrave ||
			g != nil && (aftermathHalf(k, CardID(i)) || g.altFree != nil && g.altFree[i] != 0) {
			q |= qGyCast
		}
		if g != nil && i < len(g.sCls) {
			if g.sCls[i]&sclsGyMay != 0 {
				q |= qGrave
			}
			if g.sCls[i]&sclsExMay != 0 {
				q |= qExile
			}
		}
		if pc.altCost || cd.NoManaCost || cd.Cost.Phyrexian() || cd.Convoke || cd.Delve || cd.Improvise || cd.SpellWaterbend != 0 ||
			cd.HasWarp || cd.Suspend != 0 || cd.HasWebSling || cd.HasSneak || cd.HasMayFlash || cd.HasOffering || cd.HasEvoke ||
			cd.HasDash || cd.HasBlitz || cd.HasSurge || cd.HasMadness || cd.HasMiracle || cd.Impending != 0 || cd.Teamwork != 0 ||
			cd.SelfFlash || cd.Harmonize || cd.HasModeCost || cd.HasOverload || cd.HasBestow || cd.HasMutate || cd.HasPlot || cd.HasForetell ||
			cd.MorphFamily != 0 || cd.AltCost != (kop.ManaCost{}) {
			q |= qOpen
		}
		if pc.kw&kop.KwKicker == 0 && !cd.HasMultikicker && !cd.Offspring && !cd.Buyback && !cd.Entwine && cd.Teamwork == 0 &&
			cd.Casualty == 0 && !cd.Bargain && !cd.Conspire && cd.OptCost.Flags == 0 && !cd.OptBehold && cd.OptEvidence == 0 &&
			cd.OptWaterbend == 0 && !cd.HasBestow && !cd.HasMutate && !cd.HasAltAdd && cd.Kicker == (kop.ManaCost{}) {
			q |= qPlain
		}
		qf.cast = quietFloor(&cd.Cost)
		if k.tapExtraCard[i] {
			q |= qMana
		}
		if k.grantsMana[i] || k.grantSrcCard != nil && k.grantSrcCard[i] {
			q |= qGMana
		}
		mtap, mfree := 0, 0
		for ai := range cd.Abilities {
			a := &cd.Abilities[ai]
			switch a.Kind {
			case kop.AbStatic:
				if !quietHarmless(a.Static.Kind) || g != nil && g.anyZeroAlt && a.Static.Kind == kop.StatMayPlayFrom {
					// (a may-play static's {0} alternative is a hand cast the
					// may-play zones need not name: cast_mayzeroalt.go)
					q |= qMod
					if a.Static.Kind == kop.StatReduceCost || a.Static.Kind == kop.StatSetCost || a.Static.Kind == kop.StatAlternativeCost {
						q |= qOpen // a self cost static (affinity) prices the card's own cast
					}
				}
				continue
			case kop.AbMana:
				if a.AFlags&kop.AfLatent != 0 {
					// a body a grant may hand another permanent
					if n := quietManaAmt(a); a.Cost.Flags&kop.CostTap != 0 {
						k.quietGMTap = uint8(min(max(int(k.quietGMTap), n), int(quietInf)))
					} else {
						k.quietGMTap = quietInf
					}
					continue
				}
				if a.ActZone != kop.ZoneNone && a.ActZone != kop.ZoneBattlefield {
					continue
				}
				n := quietManaAmt(a)
				switch {
				case a.Cost.Flags&kop.CostTap != 0:
					mtap = max(mtap, n)
				case a.Cost.Flags&kop.CostSacSelf != 0:
					mfree += n // once: it sacrifices the source
				case a.Cost.Flags == 0 && a.Cost.Mana.MV() >= n:
					// a filter that nets nothing: no amount beyond what pays it
				default:
					mfree = int(quietInf) // a repeatable source without {T}
				}
				continue
			case kop.AbActivated:
			default:
				continue
			}
			if sorcOnly(a) {
				continue
			}
			fl := quietAbFloor(a)
			switch a.ActZone {
			case kop.ZoneHand:
				qf.hand = min(qf.hand, fl)
			case kop.ZoneGraveyard:
				q |= qGrave
			case kop.ZoneExile:
				q |= qExile
			}
			if g == nil || ownActivatable(a) {
				if a.Cost.Flags&kop.CostTap != 0 {
					qf.bfTap = min(qf.bfTap, fl)
				} else {
					qf.bfAny = min(qf.bfAny, fl)
				}
			}
			if a.AFlags&kop.AfLatent != 0 {
				// a body some grant may hand another permanent
				if a.Cost.Flags&kop.CostTap != 0 {
					qf.grTap = min(qf.grTap, fl)
					k.quietGrantTap = min(k.quietGrantTap, fl)
				} else {
					qf.grAny = min(qf.grAny, fl)
					k.quietGrantAny = min(k.quietGrantAny, fl)
				}
			}
		}
		qf.mtap, qf.mfree = uint8(min(mtap, int(quietInf))), uint8(min(mfree, int(quietInf)))
		qf.bits = q
		k.quietCard[i] = qf
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if t.Emblems[i].Abilities[j].Kind == kop.AbActivated {
				k.quietGrantAny = 0
			}
		}
	}
}

// Quiet-proof blockers: the first route seatQuiet could not rule out.
const (
	qbNone     uint8 = iota // proved: the seat can only pass
	qbTable                 // the table carries a route the proof does not model
	qbSorc                  // the seat is at sorcery speed
	qbFlash                 // (unused: a live flash grant is read per card)
	qbHand                  // a hand card's instant-speed cast within the mana bound
	qbHandAct               // a hand card's own route ahead of the timing gate (qHand)
	qbCmd                   // a commander castable at instant speed
	qbGrave                 // a graveyard card's cast or ability
	qbExile                 // an exile card's cast or ability
	qbMayRec                // a play permission record of the seat
	qbMayZone               // a play-from-zone static in force for the seat
	qbBattle                // an activated ability of the seat's permanent within the bound
	qbGrant                 // a granted activated ability within the bound
	qbFaceDown              // a face-down permanent of the seat (turn face up)
	qbPriAsk                // an unless source's priority activation
	qbHandAb                // a hand card's activated ability (cycling) within the bound
	qbN
)

// QuietBlockerName names blocker b (verify reports).
func QuietBlockerName(b int) string {
	return [...]string{"proved", "table", "sorcery", "flash", "hand", "hand-route", "command", "graveyard",
		"exile", "may-record", "may-zone", "battlefield", "grant", "face-down", "priority-ask", "hand-ability"}[b]
}

// seatQuiet reports whether the seat holding priority provably has no option
// but pass (stepRaw's priority arm, s.g.availX == 0).
func (s *State) seatQuiet(k *prep, p uint8) bool { return s.quietBlocker(k, p) == qbNone }

// quietBound is an upper bound on the mana p could spend on one payment now,
// in any census the walk prices (the plain census, the script walk's gaps and
// any-colour censuses): the floating pool, every printed mana ability of p's
// permanents (a {T} one only untapped, at its largest amount), the hand's and
// graveyard's own mana, a basic land type's intrinsic ability on every
// untapped permanent. A granted or tap-extra mana source, restricted
// floating mana, or a dynamic amount makes it unbounded (quietInf).
func (s *State) quietBound(k *prep, p uint8) int {
	if s.rbAny(p) || k.anyCmdTap {
		return int(quietInf)
	}
	g := k.gen
	grant := k.contGrantAlways || g != nil && len(g.grantPC) != 0 && s.grantsLive(k)
	for m := s.statM; m.any(); m = m.rest() {
		b := k.quietCard[s.cardAt(m.first())].bits
		if b&qMana != 0 {
			return int(quietInf)
		}
		grant = grant || b&qGMana != 0
	}
	untapped := 0 // the least amount an untapped permanent counts
	if grant {
		if k.quietGMTap == quietInf {
			return int(quietInf)
		}
		untapped = int(k.quietGMTap)
	}
	if g != nil && (g.blgOn || g.landSet) {
		untapped = max(untapped, 1) // a basic land type's intrinsic ability
	}
	pl := &s.pl[p]
	n := 0
	for _, v := range pl.pool {
		n += int(v)
	}
	for i := 0; i < int(pl.handN); i++ {
		if k.t.Cards[s.objs[pl.hand[i]].card].HandMana != 0 {
			n++
		}
	}
	for i := 0; i < int(pl.graveN); i++ {
		n += bits.OnesCount8(k.t.Cards[s.objs[pl.grave[i]].card].GraveMana)
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		qf := &k.quietCard[s.cardAt(sl)]
		if qf.mfree == quietInf {
			return int(quietInf)
		}
		n += int(qf.mfree)
		if s.perms[sl].flags&pfTapped == 0 {
			if qf.mtap == quietInf {
				return int(quietInf)
			}
			n += max(int(qf.mtap), untapped)
		}
	}
	return n
}

// quietModLive reports whether a static the proof does not read may lower a
// payment: a qMod card on the battlefield (the cost statics' sources,
// eachStaticC) or any emblem.
func (s *State) quietModLive(k *prep) bool {
	if s.anyEmb() {
		return true
	}
	for m := s.statM; m.any(); m = m.rest() {
		if k.quietCard[s.cardAt(m.first())].bits&qMod != 0 {
			return true
		}
	}
	return false
}

// quietBlocker is seatQuiet's proof: qbNone when proved, else the first route
// it could not rule out.
func (s *State) quietBlocker(k *prep, p uint8) uint8 {
	if k.quietOff {
		return qbTable
	}
	if s.sorcerySpeed(p) {
		return qbSorc
	}
	g := k.gen
	pl := &s.pl[p]
	// the mana bound and the cost-modifier read, computed once when a floor
	// needs them (a floor of 0 is never above the bound)
	bound, mod, haveB := 0, false, false
	modLive := func() bool {
		if !haveB {
			bound, mod, haveB = s.quietBound(k, p), s.quietModLive(k), true
		}
		return mod
	}
	above := func(f uint8) bool {
		if f == quietNone {
			return true
		}
		return f != 0 && !modLive() && int(f) > bound
	}
	// flash grants (castFlash): read exactly per card only when one can be live
	flashLive := k.anyFlashAny && (k.anySelfFlash || k.anyEmbFlash && s.g.nemb[p] > 0 || k.anyFlashStat && s.quietFlashStat(k))
	inst := func(c CardID) bool {
		if k.quietCard[c].bits&qInst != 0 {
			return true
		}
		if !flashLive {
			return false
		}
		if s.castFlash(k, p, c) {
			return true
		}
		for _, f := range k.t.Cards[c].Faces {
			if s.castFlash(k, p, f) {
				return true
			}
		}
		return false
	}
	castLoud := func(c CardID) bool {
		qf := &k.quietCard[c]
		return inst(c) && (qf.bits&qOpen != 0 || !above(qf.cast))
	}
	// a hand cast of a plain card (one cast route: no kicker, optional or
	// extra cost) is offered only when its targets are feasible, a gate the
	// walk reads before every route but the plain card has none of
	handLoud := func(c CardID, o Obj) bool {
		if !castLoud(c) {
			return false
		}
		if k.quietCard[c].bits&(qOpen|qPlain) != qPlain || k.anyOffGrant || modLive() {
			return true
		}
		return s.spellTargetsFeasibleFrom(k, c, p, o)
	}
	for i := 0; i < int(pl.handN); i++ {
		c := s.objs[pl.hand[i]].card
		qf := &k.quietCard[c]
		if qf.bits&qHand != 0 {
			return qbHandAct
		}
		if !above(qf.hand) {
			return qbHandAb
		}
		if !k.cards[c].land && handLoud(c, pl.hand[i]) {
			return qbHand
		}
	}
	if s.cfg&cfgCommander != 0 {
		for _, o := range [2]Obj{pl.cmd, s.cmdObj(p, 1)} {
			if o != NoObj && s.objs[o].zone == kop.ZoneCommand && castLoud(s.objs[o].card) {
				return qbCmd
			}
		}
	}
	// the graveyard: an instant-speed card offers only through a cast route
	// (printed, a stamped or static flashback grant, a beam grant)
	var fbLo, fbHi *objBits
	if fbi := k.fbIx(); int(fbi) < len(s.cold.mo) {
		fbLo, fbHi = &s.cold.mo[fbi], &s.cold.mo[fbi+1]
	}
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		bt := k.quietCard[c].bits
		if bt&qGrave != 0 {
			return qbGrave
		}
		if !inst(c) {
			continue
		}
		if bt&qGyCast != 0 || fbLo != nil && (fbLo.has(o) || fbHi.has(o)) ||
			g != nil && (g.anyFbGrant && s.fbLive(k, o) || g.anyBeamGrant && k.t.Cards[c].Types&(kop.TInstant|kop.TSorcery) != 0) {
			return qbGrave
		}
	}
	// exile: an instant-speed card offers only through an exile cast record
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		c := s.objs[o].card
		if k.quietCard[c].bits&qExile != 0 {
			return qbExile
		}
		if !inst(c) {
			continue
		}
		if g == nil || s.objs[o].of&(ofAdventure|ofCopy) != 0 || k.t.Cards[c].Paradigm != 0 ||
			g.anyForetell && s.foretellRecord(p, o) || g.anyWarp && s.warpRecastRecord(p, o) || g.anyAirbend && s.airbendRecord(p, o) {
			return qbExile
		}
	}
	if g != nil {
		for i := 0; i < int(s.g.nmay); i++ {
			if s.mayAt(i).ctrl == p {
				return qbMayRec
			}
		}
		if g.anyMayPlay && s.mayPlayZones(k, p) != 0 {
			return qbMayZone
		}
		if s.g.priAsk != 0 && g.anyCondSplit {
			return qbPriAsk
		}
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		qf := &k.quietCard[s.cardAt(sl)]
		if !above(qf.bfAny) || s.perms[sl].flags&pfTapped == 0 && !above(qf.bfTap) {
			return qbBattle
		}
		if g != nil && g.anyFaceDown && s.faceDown(k, sl) {
			return qbFaceDown
		}
	}
	if g != nil && (k.quietGrantAny != quietNone || k.quietGrantTap != quietNone) && s.grantsLive(k) {
		// a granted body is a latent ability of a grant's source: a
		// battlefield card (a static's grant), an override record's card, or
		// an emblem's creator (any floor)
		gAny, gTap := quietNone, quietNone
		if g.anyEmbGrant && s.anyEmb() {
			gAny = 0
		}
		for m := s.statM; m.any(); m = m.rest() {
			qf := &k.quietCard[s.cardAt(m.first())]
			gAny, gTap = min(gAny, qf.grAny), min(gTap, qf.grTap)
		}
		for i := 0; i < int(s.g.nfx); i++ {
			if r := &s.g.fx[i]; r.kind == fxOverride {
				qf := &k.quietCard[r.r.src]
				gAny, gTap = min(gAny, qf.grAny), min(gTap, qf.grTap)
			}
		}
		if !above(gAny) || !above(gTap) {
			for m := s.ctrlM[p]; m.any(); m = m.rest() {
				sl := m.first()
				if s.grantMay(k, sl) && (!above(gAny) || s.perms[sl].flags&pfTapped == 0 && !above(gTap)) {
					return qbGrant
				}
			}
		}
	}
	return qbNone
}

// quietFlashStat reports whether some battlefield permanent prints a
// StatCastWithFlash static (castStatic's sources, any controller).
func (s *State) quietFlashStat(k *prep) bool {
	for m := s.statM; m.any(); m = m.rest() {
		if k.cards[s.cardAt(m.first())].flashStat {
			return true
		}
	}
	return false
}

// QuietStats is the quiet proof's verify tally (DebugQuietVerify): every posed
// priority walk runs, and the proof is read beside it.
type QuietStats struct {
	Walks, PassOnly    uint64
	Proved, ProvedPass uint64
	// Mismatch: a proved window whose walk offered more than pass; StateDiff:
	// one whose walk changed the State beyond the filter-bound memo
	// (genState.fbp, which a proved window leaves as it was: the memo's key
	// holds its inputs, so its content is never an answer's input).
	Mismatch, StateDiff uint64
	// MemoOnly: proved windows whose walk rewrote only the fbp memo.
	MemoOnly uint64
	// Blocker: pass-only windows the proof did not prove, by first blocker.
	Blocker [qbN]uint64
	// Panic: a mismatch panics (tests) instead of only counting.
	Panic bool
	// First names the first mismatch.
	First string
}

// DebugQuietVerify, when set, runs the quiet proof beside every posed priority
// walk and tallies it (the walk's answer stands). One goroutine only: the
// tally is not synchronised.
var DebugQuietVerify *QuietStats

// DebugQuietShadow, when set (and DebugQuietVerify is not), runs every posed
// priority walk and panics when the quiet proof held for a window whose walk
// offered more than pass: the oracle's lockstep games run with it on
// (dirac/oracle's TestMain), in parallel, so it keeps no tally.
var DebugQuietShadow bool

// quietShadow is the priority arm's walk under DebugQuietShadow.
func (s *State) quietShadow(k *prep, d *Decision) {
	b := s.quietBlocker(k, s.prio)
	s.priorityOptions(k, d)
	if b == qbNone && (d.N != 1 || d.Options[0].Kind() != AkPass) {
		panic(fmt.Sprintf("dirac: quiet proof wrong for seat %d (turn %d step %v stack %d): the walk offered %v, %d options",
			s.prio, s.turn, s.step, s.sp, d.Options[0], d.N))
	}
}

// quietVerify is the priority arm's walk under DebugQuietVerify.
func (s *State) quietVerify(k *prep, d *Decision, st *QuietStats) {
	p := s.prio
	b := s.quietBlocker(k, p)
	fb0 := s.g.fbp
	h0 := s.gameHash()
	s.priorityOptions(k, d)
	fb1 := s.g.fbp
	s.g.fbp = fb0
	h1 := s.gameHash()
	s.g.fbp = fb1
	st.Walks++
	pass := d.N == 1 && d.Options[0].Kind() == AkPass
	if pass {
		st.PassOnly++
	}
	if b != qbNone {
		if pass {
			st.Blocker[b]++
		}
		return
	}
	st.Proved++
	if pass {
		st.ProvedPass++
		if fb0 != fb1 && h0 == h1 {
			st.MemoOnly++
		}
	}
	if pass && h0 == h1 {
		return
	}
	what := "changed the State"
	if !pass {
		st.Mismatch++
		a := d.Options[0]
		what = fmt.Sprintf("offered %v", a)
		if src := Obj(a.Source()); a.Kind() != AkPass && int(src) < len(s.objs) {
			what += " (" + k.t.Cards[s.objs[src].card].Name + ")"
		}
	} else {
		st.StateDiff++
	}
	msg := fmt.Sprintf("dirac: quiet proof wrong for seat %d (turn %d step %v stack %d): the walk %s, %d options", p, s.turn, s.step, s.sp, what, d.N)
	if st.First == "" {
		st.First = msg
	}
	if st.Panic {
		panic(msg)
	}
}
