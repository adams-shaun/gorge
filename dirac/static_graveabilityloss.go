package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cards in graveyards lose all abilities (kop.StatGraveNoAbilities; Yixlid
// Jailer's S:Mode$ Continuous | Affected$ Card | AffectedZone$ Graveyard |
// RemoveAllAbilities$ True; CR 613.1f).
//
// gorge reads the removal live from the battlefield (rules abilityloss.go
// removalApplies, abilityLoss): a graveyard card's PRINTED abilities do not
// exist while a live removal applies to it, and an ability another effect
// GRANTED it survives only when that grant is not older than the removal
// (grantLostTo: layer 6 applies in timestamp order). The kernel keeps no
// mark on the card either. A graveyard reader asks graveAbilitiesLost for the
// removal's stamp and hides the printed ability it was about to offer, or a
// granted one older than the stamp.
//
// A grant with no stamp the kernel can compare (an Effect's or a Pump's mark:
// the mech arena planes, kw_fbgrant.go fbGranted, kw_embalmgrant.go) stalls
// by name (StallGraveLossGrant) when the card has lost its abilities: gorge
// decides by the grant's timestamp, which the mark does not carry.
//
// The readers wired: the keyword and offer readers (split 1), the trigger
// scans and the leave-graveyard look-back (split 2), and the graveyard mana
// ability (mana_gravemana.go), the printed activated-ability offer
// (legal_gen.go priorityGraveAct) and the flash of a may-play cast
// (static_graveflash.go) (split 3). A graveyard card's Continuous static
// (static_gycont.go) keeps working under the removal, as gorge's layer walk
// leaves it (pruneLostSourceAbilities is battlefield-only; gorge suspect).
//
// Where gorge applies no loss the kernel applies none either: the aftermath
// and disturb casts (legal.go aftermathAlternateFace, cast_altcost.go
// disturbCast read the printed face), a may-play permission another source
// grants (mayplay.go reads no loss gate), a hand cycling grant (the removal
// reaches only the battlefield and graveyard) and an Effect-delivered encore
// (the kernel offers none: kw_encoregrant.go).

// DebugNoGraveLoss is the mutation switch: the predicate answers "not lost",
// so a lockstep or unit test that expects the loss must fail.
var DebugNoGraveLoss bool

// GraveLossStats counts what the predicate suppressed: one hit per offer,
// keyword or trigger a graveyard reader hid because its card lost its
// abilities.
var GraveLossStats struct{ Hits atomic.Int64 }

// StallGraveLossGrant: a graveyard card that lost all abilities holds a grant
// the kernel cannot order against the removal (an Effect's or a Pump's mark
// carries no timestamp), or a printed ability beside a grant that survives
// it: gorge decides by timestamp, so the kernel stops and counts it.
const StallGraveLossGrant = 87

func init() { stallNames[StallGraveLossGrant] = "gravelossgrant" }

// useGraveLoss is the genPrep.uses bit for a table with a StatGraveNoAbilities
// static: a registered use, so the fixed State layout is unchanged
// (TestNoNewFixedStateFields) and a table without one pays one bit test.
var useGraveLoss = newGenUse(tableHasGraveLoss)

// tableHasGraveLoss reports a table that carries a StatGraveNoAbilities
// static, prepareGen's gate for useGraveLoss.
func tableHasGraveLoss(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatGraveNoAbilities {
				return true
			}
		}
	}
	return false
}

// graveLossOn reports whether the table can hold a graveyard ability loss.
func graveLossOn(k *prep) bool { return k.gen != nil && k.gen.uses.has(useGraveLoss) }

// graveAbilitiesLost reports whether graveyard card o has lost all its
// abilities: a live (statCond), phased-in battlefield StatGraveNoAbilities
// static whose source has not itself lost its abilities matches it. stamp is
// the latest such static's layer timestamp (grantTs), which the granted
// readers compare with their grant's (gorge grantLostTo: the grant survives
// unless the removal is newer).
func (s *State) graveAbilitiesLost(k *prep, o Obj) (stamp uint16, lost bool) {
	if DebugNoGraveLoss || !graveLossOn(k) || !s.statM.any() {
		return 0, false
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneGraveyard {
		return 0, false
	}
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatGraveNoAbilities || st.sl == NoSlot {
			return true
		}
		if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, ob.card, ob.owner(), st.ctrl, o, nil) {
			return true
		}
		lost = true
		if ts := s.grantTs(st); ts > stamp {
			stamp = ts
		}
		return true
	})
	return stamp, lost
}

// graveGrant describes the grants a reader's offer can stand on, beside the
// card's printed ability: none, a static grant with a layer timestamp ts, or
// a stamped one (an Effect's or a Pump's mark) the kernel cannot order.
type graveGrant struct {
	have, stamped bool
	ts            uint16
}

// graveOfferHidden reports whether the graveyard ability of o that a reader
// is about to offer is gone: printed says the offer stands on the card's own
// printed ability, g on a grant. It counts every hide in GraveLossStats and
// stalls (StallGraveLossGrant) when gorge's answer rests on an ordering the
// kernel does not hold; the caller then hides the offer too.
func (s *State) graveOfferHidden(k *prep, o Obj, printed bool, g graveGrant) bool {
	stamp, lost := s.graveAbilitiesLost(k, o)
	if !lost {
		return false
	}
	if g.have && g.stamped {
		s.capFull(StallGraveLossGrant)
		return true
	}
	survives := g.have && g.ts >= stamp // grantLostTo: only a NEWER removal ends it
	if survives && printed {
		s.capFull(StallGraveLossGrant) // the printed instance is gone, the granted one is not: gorge offers only the latter
		return true
	}
	if survives {
		return false
	}
	GraveLossStats.Hits.Add(1)
	return true
}

// graveStaticTs is the latest layer timestamp among the live statics of kind
// kind (a class cls card) that grant to card c, owned by owner and read with
// p as "you", and whether there is one. o is the card's object when the
// grant's own reader passes it to the filter.
func (s *State) graveStaticTs(k *prep, cls uint16, kind kop.StaticKind, p, owner uint8, c CardID, o Obj) (ts uint16, ok bool) {
	if k.gen == nil {
		return 0, false
	}
	s.eachStaticC(k, cls, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kind || st.ctrl != p || st.sl == NoSlot {
			return true
		}
		if cls == sclsOffKwG && a.Static.Zone != kop.ZoneAny && a.Static.Zone != kop.ZoneGraveyard {
			return true // an off-zone grant's own zone (kw_offkwgrant.go)
		}
		if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, c, owner, p, o, nil) {
			return true
		}
		ok = true
		if t := s.grantTs(st); t > ts {
			ts = t
		}
		return true
	})
	return ts, ok
}

// graveFbHidden is the flashback walk's gate (legal.go): printed is the
// card's own flashback, marked an Effect's or a Pump's grant (no stamp), and
// a StatFbGrant static is the live grant.
func (s *State) graveFbHidden(k *prep, o Obj, printed, marked bool) bool {
	ob := &s.objs[o]
	g := graveGrant{have: marked, stamped: marked}
	if !marked && k.gen.anyFbGrant {
		g.ts, g.have = s.graveStaticTs(k, sclsFbGrant, kop.StatFbGrant, ob.owner(), ob.owner(), ob.card, NoObj)
	}
	return s.graveOfferHidden(k, o, printed, g)
}

// graveEscapeHidden is the escape walk's gate (kw_escape.go): printed is the
// card's own Escape, a StatEscapeGrant static the grant.
func (s *State) graveEscapeHidden(k *prep, o Obj, printed bool) bool {
	ob := &s.objs[o]
	var g graveGrant
	if k.gen.anyEscapeGrant {
		g.ts, g.have = s.graveStaticTs(k, sclsEscapeG, kop.StatEscapeGrant, ob.owner(), ob.owner(), ob.card, NoObj)
	}
	return s.graveOfferHidden(k, o, printed, g)
}

// graveRetraceHidden is the retrace walk's gate (kw_retrace.go): printed is
// the card's own Retrace (or Jump-start), a StatRetraceGrant static the grant.
func (s *State) graveRetraceHidden(k *prep, o Obj, printed bool) bool {
	ob := &s.objs[o]
	g := graveGrant{}
	g.ts, g.have = s.graveStaticTs(k, sclsOffKwG, kop.StatRetraceGrant, ob.owner(), ob.owner(), ob.card, o)
	return s.graveOfferHidden(k, o, printed, g)
}

// graveBeamGrantTs is the latest layer timestamp among p's live beam me up
// grantors (kw_beammeup.go beamGrantor), when the table has one.
func (s *State) graveBeamGrantTs(k *prep, p uint8) (ts uint16, ok bool) {
	for m := s.statM.and(s.ctrlM[SeatIx(p)]); m.any(); m = m.rest() {
		src := m.first()
		if !s.ovrLoses(k, src) && s.hasBeamGrant(k, src) {
			ok = true
			if t := s.layerTs(src); t > ts {
				ts = t
			}
		}
	}
	return ts, ok
}

// graveBeamHidden is the beam walk's gate (kw_beammeup.go): printed says the
// card prints the keyword, granted that p's grantor gives it the second cast.
// It returns the instances that survive.
func (s *State) graveBeamHidden(k *prep, o Obj, p uint8, printed, granted bool) (printedOK, grantedOK bool) {
	var g graveGrant
	if granted {
		g.ts, g.have = s.graveBeamGrantTs(k, p)
	}
	if printed {
		printedOK = !s.graveOfferHidden(k, o, true, g)
	}
	if granted {
		grantedOK = !s.graveOfferHidden(k, o, false, g)
	}
	return printedOK, grantedOK
}

// graveUnearthHidden gates a granted unearth offer (kw_unearthgrant.go): the
// StatUnearthGrant static's source slot sl stamps the grant.
func (s *State) graveUnearthHidden(k *prep, o Obj, sl Slot) bool {
	return s.graveOfferHidden(k, o, false, graveGrant{have: true, ts: s.layerTs(sl)})
}

// graveStampedHidden gates a granted offer that stands on an Effect's mark
// (the embalm grant, kw_embalmgrant.go): the mark carries no timestamp.
func (s *State) graveStampedHidden(k *prep, o Obj) bool {
	return s.graveOfferHidden(k, o, false, graveGrant{have: true, stamped: true})
}

// gravePrintedHidden gates an offer that stands on the card's own printed
// ability alone (the graveyard activated abilities, harmonize, mayhem,
// dredge).
func (s *State) gravePrintedHidden(k *prep, o Obj) bool {
	return s.graveOfferHidden(k, o, true, graveGrant{})
}

// graveUnearthPrinted: a card that prints unearth has no granted unearth
// offer (gorge printedKeywordCovered), but once its printed ability is gone a
// grant newer than the removal is the only one left, and gorge offers it. The
// kernel does not, so it stalls there.
func (s *State) graveUnearthPrinted(k *prep, p uint8, o Obj, card CardID) {
	stamp, lost := s.graveAbilitiesLost(k, o)
	if !lost {
		return
	}
	if a, sl := s.offKwUnearthStatic(k, p, s.objs[o].owner(), card, o); a != nil && s.layerTs(sl) >= stamp {
		s.capFull(StallGraveLossGrant)
	}
}

// graveViewLost is graveAbilitiesLost for the view readers (featview.go): the
// same answer for a graveyard card, counting nothing.
func (s *State) graveViewLost(k *prep, o Obj) (stamp uint16, lost bool) {
	if !graveLossOn(k) {
		return 0, false
	}
	return s.graveAbilitiesLost(k, o)
}

// graveFbViewGrants is the number of flashback grants a graveyard card that
// lost all abilities still lists: a stamped grant (an Effect's or a Pump's
// mark, which carries no timestamp) is left out, as the walk stalls on it
// (graveFbHidden); a live static's survives when it is not older than the
// removal.
func (s *State) graveFbViewGrants(k *prep, o Obj, stamp uint16) uint8 {
	if !s.fbLive(k, o) {
		return 0
	}
	ob := &s.objs[o]
	if ts, ok := s.graveStaticTs(k, sclsFbGrant, kop.StatFbGrant, ob.owner(), ob.owner(), ob.card, NoObj); ok && ts >= stamp {
		return 1
	}
	return 0
}

// graveLeaveLost judges the loss for a graveyard card o about to leave for
// zone to, as gorge's trigger walk does (trigger_match.go lossObj; CR
// 603.10a): an ability that triggers when a card leaves a graveyard looks
// back at the card as it was before the move, so the answer is read while o
// is still in the graveyard. A card moving to the battlefield is judged on
// the new object, which no graveyard removal reaches. Call it BEFORE detach.
func (s *State) graveLeaveLost(k *prep, o Obj, to kop.Zone) (stamp uint16, lost bool) {
	if to == kop.ZoneBattlefield {
		return 0, false
	}
	return s.graveAbilitiesLost(k, o)
}
