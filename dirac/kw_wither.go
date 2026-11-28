package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted wither (CR 702.80): a Pump KW$ Wither, a
// Continuous static's AddKeyword$ Wither or an override's AddKw carries the
// keyword bit kop.KwWither (the old reserved KwShadowNone). Printed wither
// stays CardDef.Wither, which kgen never turns into a bit. gorge's damage
// paths read the DERIVED keywords (effects/damage.go HasKeyword(source,
// "Wither")), so a granted wither deals -1/-1 counters to creatures exactly as
// a printed one does (the infect creature half: kw_infect.go).

// DebugWitherGrantIgnored is the mutation switch: witherKw ignores the
// granted bit (TestWitherMutation expects the lockstep to diverge).
var DebugWitherGrantIgnored bool

// witherKw: the permanent at sl has wither -- printed and not lost
// (infectAt's rule), or granted (kw is sl's current keywords; kw() applies
// the layer-6 loss).
func (s *State) witherKw(k *prep, sl Slot, kw kop.KwMask) bool {
	if s.pilePrints(k, sl, printsWither) && !s.genLost(k, sl) {
		return true
	}
	return k.gen.anyWitherGrant && !DebugWitherGrantIgnored && kw&kop.KwWither != 0
}

// witherAt is witherKw for a caller without the keywords in hand.
func (s *State) witherAt(k *prep, sl Slot) bool {
	if s.pilePrints(k, sl, printsWither) && !s.genLost(k, sl) {
		return true
	}
	return k.gen.anyWitherGrant && s.witherKw(k, sl, s.kw(k, sl))
}

func printsWither(cd *kop.CardDef) bool { return cd.Wither }

// srcWither is the resolving source's wither (kw: srcKw's result): the
// permanent's while it is on the battlefield; else gorge's LKI wither
// (effects/damage.go resolveDamageRider: lki.Wither, derived at departure)
// when the entry holds the source's snapshot, else its card's printed wither.
func (s *State) srcWither(k *prep, kw kop.KwMask) bool {
	if sl := s.objRef(k, kop.RefSelf); sl != NoSlot {
		return s.witherKw(k, sl, kw)
	}
	e := &s.res.e
	if k.t.Cards[e.card].Wither {
		return true
	}
	if e.kind != seSpell {
		if e.li != 0 {
			if l := s.lk(e.li); l.obj == e.obj && s.lkiWithered(k, l) {
				return true
			}
		}
		if l := s.srcDepartRec(e); l != nil && s.lkiWithered(k, l) {
			return true
		}
	}
	return false
}

// lkiWithered: the snapshot l has wither -- the printed one derived at
// departure over the whole pile (the lkiWither bit, gorge's lki.Wither,
// CR 113.7a), or a granted one (l.kw holds only that keyword bit,
// kw_wither.go's granted rule).
func (s *State) lkiWithered(k *prep, l *lkiRec) bool {
	return l.bits&lkiWither != 0 ||
		k.gen != nil && k.gen.anyWitherGrant && !DebugWitherGrantIgnored && l.kw&kop.KwWither != 0
}

// tableGrantsWither reports whether some part of t can grant wither
// (tableGrantsKw).
func tableGrantsWither(t *kop.Table) bool { return tableGrantsKw(t, kop.KwWither) }
