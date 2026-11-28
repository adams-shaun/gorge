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
	// gen: the cast's generic part; red: the generic a battlefield
	// ReduceCost static of this card takes off another spell's cost, summed
	// over its statics (quietInf: a dynamic or coloured reduction)
	gen, red uint8
	// mcol: the colours (W U B R G C bits) its printed mana abilities can
	// make, every colour for a chosen, reflected, dynamic or any-colour one;
	// fcol: the colours a filter of its (no {T}, it nets nothing) converts to
	mcol, fcol uint8
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
	// another payment (an alternative cost, a stack keyword grant, an ability
	// reduction): while one is on the battlefield every floor reads 0.
	qMod
	// qRed: the card prints a battlefield ReduceCost static, read as an
	// amount (quietFact.red) off cast floors only.
	qRed
	// qAnyCol: the card prints a static that lets mana of any colour (or
	// type) pay a pip: while one is on the battlefield no colour is read.
	qAnyCol
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
	tf := quietTableFlags{equipFlash: g != nil && g.uses.has(useEquipFlash), loyFlash: tableHasStatic(t, kop.StatLoyaltyFlash)}
	for i := range t.Cards {
		qf := quietFact{bfAny: quietNone, bfTap: quietNone, hand: quietNone, grAny: quietNone, grTap: quietNone}
		qf.bits = quietCardBits(t, k, i)
		qf.cast, qf.gen = quietFloor(&t.Cards[i].Cost), t.Cards[i].Cost.Generic
		quietAbilities(t, k, i, tf, &qf)
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

// quietTableFlags is the table statics that move an ability's timing.
type quietTableFlags struct{ equipFlash, loyFlash bool }

// sorcOnly reports whether activated ability a is sorcery-speed on the table:
// CostSorcery without an equip-flash static, CostLoyalty without a
// loyalty-flash one.
func (tf quietTableFlags) sorcOnly(a *kop.Ability) bool {
	f := a.Cost.Flags
	return f&kop.CostSorcery != 0 && !(tf.equipFlash && a.AFlags&kop.AfEquip != 0) || f&kop.CostLoyalty != 0 && !tf.loyFlash
}

// quietCardBits is card i's route bits read off its CardDef and prep record.
func quietCardBits(t *kop.Table, k *prep, i int) uint16 {
	cd := &t.Cards[i]
	pc := &k.cards[i]
	g := k.gen
	q := uint16(0)
	if quietInstant(k, cd, pc) {
		q |= qInst
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
	if quietOpenCast(cd, pc) {
		q |= qOpen
	}
	if quietPlainCast(cd, pc) {
		q |= qPlain
	}
	if i < len(k.tapExtraCard) && k.tapExtraCard[i] {
		q |= qMana
	}
	if i < len(k.grantsMana) && k.grantsMana[i] || i < len(k.grantSrcCard) && k.grantSrcCard[i] {
		q |= qGMana
	}
	return q
}

// quietInstant: the card or one of its faces is an instant or has flash.
func quietInstant(k *prep, cd *kop.CardDef, pc *pcard) bool {
	if pc.instant || pc.kw&kop.KwFlash != 0 {
		return true
	}
	for _, f := range cd.Faces {
		if int(f) < len(k.cards) && (k.cards[f].instant || k.cards[f].kw&kop.KwFlash != 0) {
			return true
		}
	}
	return false
}

// quietOpenCast: the card's cast price is not bounded below by its mana cost.
func quietOpenCast(cd *kop.CardDef, pc *pcard) bool {
	return pc.altCost || cd.NoManaCost || cd.Cost.Phyrexian() || cd.Convoke || cd.Delve || cd.Improvise || cd.SpellWaterbend != 0 ||
		cd.HasWarp || cd.Suspend != 0 || cd.HasWebSling || cd.HasSneak || cd.HasMayFlash || cd.HasOffering || cd.HasEvoke ||
		cd.HasDash || cd.HasBlitz || cd.HasSurge || cd.HasMadness || cd.HasMiracle || cd.Impending != 0 || cd.Teamwork != 0 ||
		cd.SelfFlash || cd.Harmonize || cd.HasModeCost || cd.HasOverload || cd.HasBestow || cd.HasMutate || cd.HasPlot || cd.HasForetell ||
		cd.MorphFamily != 0 || cd.AltCost != (kop.ManaCost{})
}

// quietPlainCast: the card's hand cast has one route (qPlain).
func quietPlainCast(cd *kop.CardDef, pc *pcard) bool {
	return pc.kw&kop.KwKicker == 0 && !cd.HasMultikicker && !cd.Offspring && !cd.Buyback && !cd.Entwine && cd.Teamwork == 0 &&
		cd.Casualty == 0 && !cd.Bargain && !cd.Conspire && cd.OptCost.Flags == 0 && !cd.OptBehold && cd.OptEvidence == 0 &&
		cd.OptWaterbend == 0 && !cd.HasBestow && !cd.HasMutate && !cd.HasAltAdd && cd.Kicker == (kop.ManaCost{})
}

// quietAbilities folds card i's abilities into qf: its statics' qMod, its
// mana amounts, its activated abilities' floors, and the table's grant bodies.
func quietAbilities(t *kop.Table, k *prep, i int, tf quietTableFlags, qf *quietFact) {
	cd := &t.Cards[i]
	g := k.gen
	mtap, mfree := 0, 0
	for ai := range cd.Abilities {
		a := &cd.Abilities[ai]
		switch a.Kind {
		case kop.AbStatic:
			if a.Static.Kind == kop.StatReduceCost {
				quietReduce(a, qf)
				continue
			}
			if a.Static.Kind == kop.StatAnyColorAll || a.Static.Kind == kop.StatAnyColorSpells || a.Static.Kind == kop.StatAnyTypeCreatures {
				qf.bits |= qAnyCol
			}
			if !quietHarmless(a.Static.Kind) || g != nil && g.anyZeroAlt && a.Static.Kind == kop.StatMayPlayFrom {
				// (a may-play static's {0} alternative is a hand cast the
				// may-play zones need not name: cast_mayzeroalt.go)
				qf.bits |= qMod
				if a.Static.Kind == kop.StatSetCost || a.Static.Kind == kop.StatAlternativeCost {
					qf.bits |= qOpen // a self cost static (affinity) prices the card's own cast
				}
			}
		case kop.AbMana:
			quietManaAbility(k, a, &mtap, &mfree, qf)
		case kop.AbActivated:
			if !tf.sorcOnly(a) {
				quietActivated(k, a, qf)
			}
		}
	}
	qf.mtap, qf.mfree = uint8(min(mtap, int(quietInf))), uint8(min(mfree, int(quietInf)))
}

// quietReduce folds a ReduceCost static into qf. One that works only on its
// own card (Zone ZoneCommand: affinity, Ghalta's) opens that card's own cast
// and lowers nothing else (adjustCostGenB reads it only for the card being
// cast; eachStaticC leaves it out). A battlefield one lowers another spell's
// generic part by its constant amount; a dynamic, emblem-numbered or
// coloured one by an unbounded amount.
func quietReduce(a *kop.Ability, qf *quietFact) {
	st := &a.Static
	if st.Zone == kop.ZoneCommand {
		qf.bits |= qOpen
		return
	}
	if st.StFlags&kop.StReduceColor != 0 {
		qf.bits |= qMod // it lowers pips: no floor or colour read holds
		return
	}
	qf.bits |= qRed
	n := int(st.A)
	if st.StFlags&(kop.StDynamic|kop.StEmbNum) != 0 {
		n = int(quietInf)
	}
	qf.red = uint8(min(int(qf.red)+n, int(quietInf)))
}

// quietManaAbility folds mana ability a into a card's amounts (mtap, mfree),
// or a latent one into the table's granted {T} amount.
func quietManaAbility(k *prep, a *kop.Ability, mtap, mfree *int, qf *quietFact) {
	if a.AFlags&kop.AfLatent != 0 {
		// a body a grant may hand another permanent
		if n := quietManaAmt(a); a.Cost.Flags&kop.CostTap != 0 {
			k.quietGMTap = uint8(min(max(int(k.quietGMTap), n), int(quietInf)))
		} else {
			k.quietGMTap = quietInf
		}
		return
	}
	if a.ActZone != kop.ZoneNone && a.ActZone != kop.ZoneBattlefield {
		return
	}
	n := quietManaAmt(a)
	col := quietManaCols(a)
	qf.mcol |= col
	switch {
	case a.Cost.Flags&kop.CostTap != 0:
		*mtap = max(*mtap, n)
	case a.Cost.Flags&kop.CostSacSelf != 0:
		*mfree += n // once: it sacrifices the source
	case a.Cost.Flags == 0 && a.Cost.Mana.MV() >= n:
		// a filter that nets nothing: no amount beyond what pays it, but it
		// turns other mana into its colours
		qf.fcol |= col
	default:
		*mfree = int(quietInf) // a repeatable source without {T}
	}
}

// quietAllCols is every colour bit, colourless included.
const quietAllCols uint8 = 0x3f

// quietManaCols is the colours mana ability a can make: its ManaColors, every
// colour for a chosen, reflected, each-colour, identity or dynamic one or an
// empty mask, colourless with any colour (an over-count).
func quietManaCols(a *kop.Ability) uint8 {
	c := uint8(a.ManaColors) & quietAllCols
	if c == 0 || c&uint8(kop.ColAny) == uint8(kop.ColAny) ||
		a.AFlags&(kop.AfManaChosen|kop.AfManaReflect|kop.AfManaEachColor|kop.AfManaDynamic) != 0 || a.ManaIdentity || a.ManaProduce {
		return quietAllCols
	}
	return c
}

// quietActivated folds a non-sorcery activated ability into qf's floors and
// zone bits, and a latent one into the grant bodies' floors.
func quietActivated(k *prep, a *kop.Ability, qf *quietFact) {
	fl := quietAbFloor(a)
	switch a.ActZone {
	case kop.ZoneHand:
		qf.hand = min(qf.hand, fl)
	case kop.ZoneGraveyard:
		qf.bits |= qGrave
	case kop.ZoneExile:
		qf.bits |= qExile
	}
	tap := a.Cost.Flags&kop.CostTap != 0
	if k.gen == nil || ownActivatable(a) {
		if tap {
			qf.bfTap = min(qf.bfTap, fl)
		} else {
			qf.bfAny = min(qf.bfAny, fl)
		}
	}
	if a.AFlags&kop.AfLatent != 0 {
		// a body some grant may hand another permanent
		if tap {
			qf.grTap = min(qf.grTap, fl)
			k.quietGrantTap = min(k.quietGrantTap, fl)
		} else {
			qf.grAny = min(qf.grAny, fl)
			k.quietGrantAny = min(k.quietGrantAny, fl)
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
func (s *State) quietBound(k *prep, p uint8, col *[kop.NumManaKinds]int) int {
	inf := int(quietInf)
	if s.rbAny(p) || k.anyCmdTap {
		return inf
	}
	g := k.gen
	grant := k.contGrantAlways || g != nil && len(g.grantPC) != 0 && s.grantsLive(k)
	anyCol := false // a static lets mana of any colour pay a pip: colours are not read
	for m := s.statM; m.any(); m = m.rest() {
		b := k.quietCard[s.cardAt(m.first())].bits
		if b&qMana != 0 {
			return inf
		}
		grant = grant || b&qGMana != 0
		anyCol = anyCol || b&qAnyCol != 0
	}
	untapped := 0 // the least amount an untapped permanent counts, of any colour
	if grant {
		if k.quietGMTap == quietInf {
			return inf
		}
		untapped = int(k.quietGMTap)
	}
	if g != nil && (g.blgOn || g.landSet) {
		untapped = max(untapped, 1) // a basic land type's intrinsic ability
	}
	pl := &s.pl[p]
	n := 0
	for i, v := range pl.pool {
		n += int(v)
		col[i] += int(v)
	}
	for i := 0; i < int(pl.handN); i++ {
		if m := k.t.Cards[s.objs[pl.hand[i]].card].HandMana; m != 0 {
			n++
			quietAddCols(col, m, 1)
		}
	}
	for i := 0; i < int(pl.graveN); i++ {
		m := k.t.Cards[s.objs[pl.grave[i]].card].GraveMana
		n += bits.OnesCount8(m)
		quietAddCols(col, m, bits.OnesCount8(m))
	}
	conv := uint8(0) // the colours a filter can turn mana into
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		qf := &k.quietCard[s.cardAt(sl)]
		if qf.mfree == quietInf {
			return inf
		}
		a := int(qf.mfree)
		mc := qf.mcol
		if s.perms[sl].flags&pfTapped == 0 {
			if qf.mtap == quietInf {
				return inf
			}
			a += max(int(qf.mtap), untapped)
			if untapped > 0 {
				mc = quietAllCols
			}
		}
		n += a
		quietAddCols(col, mc, a)
		conv |= qf.fcol
	}
	if anyCol {
		conv = quietAllCols
	}
	for c := range col {
		if conv&(1<<c) != 0 {
			col[c] = n
		}
	}
	return n
}

// quietAddCols adds n to every colour of mask m.
func quietAddCols(col *[kop.NumManaKinds]int, m uint8, n int) {
	for c := range col {
		if m&(1<<c) != 0 {
			col[c] += n
		}
	}
}

// quietModLive reports whether a static the proof does not read may lower a
// payment: a qMod card on the battlefield (the cost statics' sources,
// eachStaticC) or any emblem; else red, the most generic mana the
// battlefield's ReduceCost statics can take off a spell's cost (any
// controller's: an over-count).
func (s *State) quietModLive(k *prep) (mod bool, red int) {
	if s.anyEmb() {
		return true, 0
	}
	for m := s.statM; m.any(); m = m.rest() {
		qf := &k.quietCard[s.cardAt(m.first())]
		if qf.bits&qMod != 0 {
			return true, 0
		}
		if qf.bits&qRed != 0 {
			red = min(red+int(qf.red), int(quietInf))
		}
	}
	return false, red
}

// quietBlocker is seatQuiet's proof: qbNone when proved, else the first route
// it could not rule out.
func (s *State) quietBlocker(k *prep, p uint8) uint8 {
	b, _ := s.quietBlockerWhy(k, p)
	return b
}

// Why a floor read failed (quietRead.why): the verify's detail tally.
const (
	qwNone    uint8 = iota
	qwZero          // the floor is 0 (a free ability or cost)
	qwEmblem        // an emblem makes every floor 0
	qwMod           // a battlefield cost-modifying static makes every floor 0
	qwInf           // the mana bound is unbounded
	qwUnder         // the floor is within a finite bound
	qwOpen          // the cast's price is open (qOpen)
	qwTargets       // a plain cast's targets are feasible
	qwColour        // (a read that held: a pip colour the seat cannot make)
	qwN
)

// QuietWhyName names why detail w.
func QuietWhyName(w int) string {
	return [...]string{"-", "floor-0", "emblem", "cost-static", "bound-inf", "under-bound", "open-price", "targets-ok", "colour"}[w]
}

// quietBlockerWhy is quietBlocker with why its deciding floor read failed.
func (s *State) quietBlockerWhy(k *prep, p uint8) (uint8, uint8) {
	b, r := s.quietProof(k, p)
	if st := DebugQuietVerify; st != nil && st.OnBlock != nil && b != qbNone {
		st.OnBlock(s, b, r.why, k.t.Cards[r.card].Name, r.bound)
	}
	return b, r.why
}

func (s *State) quietProof(k *prep, p uint8) (uint8, quietRead) {
	if k.quietOff {
		return qbTable, quietRead{}
	}
	if s.sorcerySpeed(p) {
		return qbSorc, quietRead{}
	}
	// flash grants (castFlash): read exactly per card only when one can be live
	r := quietRead{s: s, k: k, p: p,
		flashLive: k.anyFlashAny && (k.anySelfFlash || k.anyEmbFlash && s.g.nemb[p] > 0 || k.anyFlashStat && s.quietFlashStat(k))}
	if b := r.hand(); b != qbNone {
		return b, r
	}
	if b := r.zones(); b != qbNone {
		return b, r
	}
	if b := r.records(); b != qbNone {
		return b, r
	}
	if b := r.battle(); b != qbNone {
		return b, r
	}
	b := r.grants()
	return b, r
}

// quietRead is one proof's reads: the mana bound and the cost-modifier read,
// computed once when a floor needs them (a floor of 0 is never above the
// bound), and whether a flash grant can be live.
type quietRead struct {
	s                     *State
	k                     *prep
	p                     uint8
	bound, red            int
	col                   [kop.NumManaKinds]int // the bound per colour (quietBound)
	mod, haveB, flashLive bool
	why                   uint8  // the last failed floor read (qw*)
	card                  CardID // the blocking card (hand and battlefield reads; verify reports)
}

func (r *quietRead) modLive() bool {
	if !r.haveB {
		r.bound, r.haveB = r.s.quietBound(r.k, r.p, &r.col), true
		r.mod, r.red = r.s.quietModLive(r.k)
	}
	return r.mod
}

// above reports whether floor f is above every mana the seat could pay with.
func (r *quietRead) above(f uint8) bool {
	if f == quietNone {
		return true
	}
	switch {
	case f == 0:
		r.why = qwZero
	case r.modLive():
		r.why = qwMod
		if r.s.anyEmb() {
			r.why = qwEmblem
		}
	case int(f) > r.bound:
		return true
	case r.bound >= int(quietInf):
		r.why = qwInf
	default:
		r.why = qwUnder
	}
	return false
}

// inst reports whether card c (or a face) can be cast at instant speed now.
func (r *quietRead) inst(c CardID) bool {
	if r.k.quietCard[c].bits&qInst != 0 {
		return true
	}
	if !r.flashLive {
		return false
	}
	if r.s.castFlash(r.k, r.p, c) {
		return true
	}
	for _, f := range r.k.t.Cards[c].Faces {
		if r.s.castFlash(r.k, r.p, f) {
			return true
		}
	}
	return false
}

// castLoud: c is castable at instant speed and its cast floor, less the
// generic the live ReduceCost statics take, is within the bound.
func (r *quietRead) castLoud(c CardID) bool {
	qf := &r.k.quietCard[c]
	if !r.inst(c) {
		return false
	}
	if qf.bits&qOpen != 0 {
		return true
	}
	f := qf.cast
	if f != 0 && !r.modLive() && r.red > 0 {
		f -= uint8(min(r.red, int(qf.gen)))
	}
	if r.above(f) {
		return false
	}
	return !r.colShort(&r.k.t.Cards[c].Cost.Pips)
}

// colShort reports whether some coloured pip count is above every mana of
// that colour the seat could make (the bound read; no cost static live).
func (r *quietRead) colShort(pips *[kop.NumManaKinds]uint8) bool {
	if !r.haveB || r.mod || r.bound >= int(quietInf) {
		return false
	}
	for c, n := range pips {
		if int(n) > r.col[c] {
			r.why = qwColour
			return true
		}
	}
	return false
}

// handLoud: a hand cast of a plain card (one cast route: no kicker, optional
// or extra cost) is offered only when its targets are feasible, a gate the
// walk reads before every route but the plain card has none of.
func (r *quietRead) handLoud(c CardID, o Obj) bool {
	if !r.castLoud(c) {
		return false
	}
	if r.k.quietCard[c].bits&(qOpen|qPlain) != qPlain || r.k.anyOffGrant || r.modLive() {
		switch {
		case r.k.quietCard[c].bits&qOpen != 0:
			r.why = qwOpen
		case r.modLive():
			r.why = qwMod
			if r.s.anyEmb() {
				r.why = qwEmblem
			}
		}
		return true
	}
	if r.s.spellTargetsFeasibleFrom(r.k, c, r.p, o) {
		r.why = qwTargets
		return true
	}
	return false
}

// hand is the hand's and the command zone's routes.
func (r *quietRead) hand() uint8 {
	s, k := r.s, r.k
	pl := &s.pl[r.p]
	for i := 0; i < int(pl.handN); i++ {
		c := s.objs[pl.hand[i]].card
		qf := &k.quietCard[c]
		if qf.bits&qHand != 0 {
			return qbHandAct
		}
		if !r.above(qf.hand) {
			return qbHandAb
		}
		if !k.cards[c].land && r.handLoud(c, pl.hand[i]) {
			r.card = c
			return qbHand
		}
	}
	if s.cfg&cfgCommander != 0 {
		for _, o := range [2]Obj{pl.cmd, s.cmdObj(r.p, 1)} {
			if o != NoObj && s.objs[o].zone == kop.ZoneCommand && r.castLoud(s.objs[o].card) {
				return qbCmd
			}
		}
	}
	return qbNone
}

// zones is the graveyard's and exile's routes.
func (r *quietRead) zones() uint8 {
	if b := r.grave(); b != qbNone {
		return b
	}
	return r.exile()
}

// grave: an instant-speed graveyard card offers only through a cast route
// (printed, a stamped or static flashback grant, a beam grant).
func (r *quietRead) grave() uint8 {
	s, k, g := r.s, r.k, r.k.gen
	pl := &s.pl[r.p]
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
		if !r.inst(c) {
			continue
		}
		if bt&qGyCast != 0 || fbLo != nil && (fbLo.has(o) || fbHi.has(o)) ||
			g != nil && (g.anyFbGrant && s.fbLive(k, o) || g.anyBeamGrant && k.t.Cards[c].Types&(kop.TInstant|kop.TSorcery) != 0) {
			return qbGrave
		}
	}
	return qbNone
}

// exile: an instant-speed exile card offers only through an exile cast record.
func (r *quietRead) exile() uint8 {
	s, k, g, p := r.s, r.k, r.k.gen, r.p
	pl := &s.pl[p]
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		c := s.objs[o].card
		if k.quietCard[c].bits&qExile != 0 {
			return qbExile
		}
		if !r.inst(c) {
			continue
		}
		if g == nil || s.objs[o].of&(ofAdventure|ofCopy) != 0 || k.t.Cards[c].Paradigm != 0 ||
			g.anyForetell && s.foretellRecord(p, o) || g.anyWarp && s.warpRecastRecord(p, o) || g.anyAirbend && s.airbendRecord(p, o) {
			return qbExile
		}
	}
	return qbNone
}

// records is the play permissions, the may-play statics and the unless
// source's priority activation.
func (r *quietRead) records() uint8 {
	s, k, g := r.s, r.k, r.k.gen
	if g == nil {
		return qbNone
	}
	for i := 0; i < int(s.g.nmay); i++ {
		if s.mayAt(i).ctrl == r.p {
			return qbMayRec
		}
	}
	if g.anyMayPlay && s.mayPlayZones(k, r.p) != 0 {
		return qbMayZone
	}
	if s.g.priAsk != 0 && g.anyCondSplit {
		return qbPriAsk
	}
	return qbNone
}

// battle is the seat's permanents' own activated abilities and face-down
// permanents.
func (r *quietRead) battle() uint8 {
	s, k, g := r.s, r.k, r.k.gen
	for m := s.ctrlM[r.p]; m.any(); m = m.rest() {
		sl := m.first()
		qf := &k.quietCard[s.cardAt(sl)]
		if (!r.above(qf.bfAny) || s.perms[sl].flags&pfTapped == 0 && !r.above(qf.bfTap)) && r.bfLive(sl) {
			r.card = s.cardAt(sl)
			return qbBattle
		}
		if g != nil && g.anyFaceDown && s.faceDown(k, sl) {
			return qbFaceDown
		}
	}
	return qbNone
}

// bfLive is the battlefield read ability by ability, for a permanent whose
// least floor is within the bound: an own non-sorcery activated ability is
// offered only through canActivate (activateOffers, and the script walk's
// nested walks), so each of canActivate's census-free gates is a necessary
// condition: the floor and its colours within the bound, an untapped and
// unsick source for {T}, the counters a removal costs, the ability's gate,
// and a feasible target.
func (r *quietRead) bfLive(sl Slot) bool {
	s, k := r.s, r.k
	pm := &s.perms[sl]
	cd := &k.t.Cards[s.cardAt(sl)]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbActivated || k.gen != nil && !ownActivatable(a) || k.quietTF.sorcOnly(a) {
			continue
		}
		fl := quietAbFloor(a)
		if r.above(fl) || fl != 0 && r.colShort(&a.Cost.Mana.Pips) {
			continue
		}
		f := a.Cost.Flags
		if f&kop.CostTap != 0 && (pm.flags&pfTapped != 0 || s.activationSick(k, sl)) {
			continue
		}
		if f&kop.CostTap != 0 && fl != 0 && r.ownTapFunds(sl, fl) {
			continue // the source's own tap cannot fund its {T} ability's mana (abilityAffords)
		}
		if f&kop.CostRemoveCtr != 0 && pm.ctrN(s, int(a.Cost.Ctr)) < int(a.Cost.N) {
			continue
		}
		if f&kop.CostSac != 0 && s.countCostSac(k, pm.ctrl, &a.Cost, sl) < int(a.Cost.N) {
			continue
		}
		if f&kop.CostOncePerTurn != 0 && i < 8 && pm.actUsed&(1<<i) != 0 {
			continue
		}
		if a.Trig.Cond != 0 && !s.activateCondHolds(k, sl, a) {
			continue
		}
		if k.anyGate && !s.gateOK(sl, a) {
			continue
		}
		if !(a.Cost.Mana.X == 0 && announcedCountX(k, &a.Cost)) && !s.abilityTargetsFeasible(k, a, pm.ctrl, sl, 0xff) {
			continue
		}
		r.why = qwUnder
		if fl == 0 {
			r.why = qwZero
		}
		return true
	}
	return false
}

// ownTapFunds reports whether floor fl is above the bound less what the
// untapped source at sl itself counted for one {T}: an ability that taps
// its source cannot spend that tap's mana.
func (r *quietRead) ownTapFunds(sl Slot, fl uint8) bool {
	if r.bound >= int(quietInf) || r.mod {
		return false
	}
	own := int(r.k.quietCard[r.s.cardAt(sl)].mtap)
	return int(fl) > r.bound-own
}

// grants: a granted body is a latent ability of a grant's source: a
// battlefield card (a static's grant), an override record's card, or an
// emblem's creator (any floor).
func (r *quietRead) grants() uint8 {
	s, k, g := r.s, r.k, r.k.gen
	if g == nil || k.quietGrantAny == quietNone && k.quietGrantTap == quietNone || !s.grantsLive(k) {
		return qbNone
	}
	gAny, gTap := quietNone, quietNone
	if g.anyEmbGrant && s.anyEmb() {
		gAny = 0
	}
	for m := s.statM; m.any(); m = m.rest() {
		qf := &k.quietCard[s.cardAt(m.first())]
		gAny, gTap = min(gAny, qf.grAny), min(gTap, qf.grTap)
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if fx := &s.g.fx[i]; fx.kind == fxOverride {
			qf := &k.quietCard[fx.r.src]
			gAny, gTap = min(gAny, qf.grAny), min(gTap, qf.grTap)
		}
	}
	if r.above(gAny) && r.above(gTap) {
		return qbNone
	}
	for m := s.ctrlM[r.p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.grantMay(k, sl) && (!r.above(gAny) || s.perms[sl].flags&pfTapped == 0 && !r.above(gTap)) {
			return qbGrant
		}
	}
	return qbNone
}

// quietModSource reports the first live cost-modifying static to f (verify).
func (s *State) quietModSource(k *prep, f func(string, kop.StaticKind)) {
	for m := s.statM; m.any(); m = m.rest() {
		c := s.cardAt(m.first())
		if k.quietCard[c].bits&qMod == 0 {
			continue
		}
		for i := range k.t.Cards[c].Abilities {
			if a := &k.t.Cards[c].Abilities[i]; a.Kind == kop.AbStatic && (!quietHarmless(a.Static.Kind) || a.Static.Kind == kop.StatMayPlayFrom) {
				f(k.t.Cards[c].Name, a.Static.Kind)
				return
			}
		}
	}
}

// priorityArm is stepRaw's posed priority options: pass alone for a seat the
// proof shows can only pass, else the walk (under DebugQuietVerify or
// DebugQuietShadow the walk always runs, beside the proof).
func (s *State) priorityArm(k *prep, d *Decision) {
	switch {
	case DebugQuietVerify != nil:
		s.quietVerify(k, d, DebugQuietVerify)
	case DebugQuietShadow:
		s.quietShadow(k, d)
	case s.seatQuiet(k, s.prio):
		d.add(MakeAction(AkPass, 0, 0, 0)) // the walk's answer, unwalked
	default:
		s.priorityOptions(k, d)
	}
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
	// Blocker: pass-only windows the proof did not prove, by first blocker;
	// Why: the same by first blocker and why its floor read failed.
	Blocker [qbN]uint64
	Why     [qbN][qwN]uint64
	// OnMod, when set, is told the first cost-modifying static (its card
	// and kind) behind each pass-only window a live modifier left unproved.
	OnMod func(card string, kind kop.StaticKind)
	// OnBlock, when set, is told each unproved window's blocker, why, the
	// blocking card (hand and battlefield blockers) and the mana bound.
	OnBlock func(s *State, b, why uint8, card string, bound int)
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
	b, why := s.quietBlockerWhy(k, p)
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
			st.Why[b][why]++
			if why == qwMod && st.OnMod != nil {
				s.quietModSource(k, st.OnMod)
			}
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
