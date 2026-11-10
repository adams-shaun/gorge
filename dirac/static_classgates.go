package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Static-class gates (kprio): per card, which classes of static ability it
// prints, so a walk over the statics in force that only acts on one class
// skips a permanent without one before asking whether it lost its
// abilities. The walk visits the same statics in the same order; only
// permanents whose every ability the caller would ignore are skipped.
const (
	sclsPT      uint16 = 1 << iota // StatContinuous with a P/T delta (ptGen)
	sclsKw                         // StatContinuous granting keywords (kwGen)
	sclsSub                        // StatContinuous granting subtypes
	sclsCont                       // any StatContinuous
	sclsMayPlay                    // StatMayPlayFrom off the battlefield's permanents
	sclsReduce                     // StatReduceCost from a permanent (Zone != ZoneCommand)
	sclsGyMay                      // the card's own StatMayPlayFrom that works from its graveyard
	sclsPTRel                      // a static basePTGen/ptGen can read: P/T delta, CDA P/T, override, subtype grant (kprio2 sbaPTQuiet)
	sclsExMay                      // the card's own StatMayPlayFrom that works from exile
	sclsFbGrant                    // StatFbGrant (kw_fbgrant.go)
	sclsWarpG                      // StatWarpGrant (kw_warpgrant.go)
)

// prepStaticClasses fills g.sCls.
func prepStaticClasses(t *kop.Table, g *genPrep) {
	g.sCls = make([]uint16, len(t.Cards))
	for i := range t.Cards {
		c := &t.Cards[i]
		m := uint16(0)
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind != kop.AbStatic {
				continue
			}
			sa := &a.Static
			switch sa.Kind {
			case kop.StatContinuous:
				m |= sclsCont
				if sa.StFlags&kop.StDynamic != 0 || sa.A != 0 || sa.B != 0 {
					m |= sclsPT | sclsPTRel
				}
				if sa.Kw != 0 {
					m |= sclsKw
				}
				if sa.AddSub != (kop.SubMask{}) || sa.AddChosen {
					m |= sclsSub | sclsPTRel
				}
			case kop.StatCDAPT, kop.StatOverride:
				m |= sclsPTRel
			case kop.StatMayPlayFrom:
				switch {
				case sa.StFlags&kop.StFunctionsInGraveyard != 0:
					m |= sclsGyMay
				case sa.StFlags&kop.StFunctionsInExile != 0:
					m |= sclsExMay
				default:
					m |= sclsMayPlay
				}
			case kop.StatReduceCost:
				if sa.Zone != kop.ZoneCommand {
					m |= sclsReduce
				}
			case kop.StatReduceAbility:
				m |= sclsReduce // cost_reduceability.go
			case kop.StatFbGrant:
				m |= sclsFbGrant
			case kop.StatWarpGrant:
				m |= sclsWarpG
			}
		}
		g.sCls[i] = m
	}
}

// eachStaticC is eachStatic over the permanents whose card prints a static
// of a class in cls (emblems all walked): f sees exactly the statics of
// those classes eachStatic would show it, in the same order.
func (s *State) eachStaticC(k *prep, cls uint16, f func(st statSrc) bool) {
	for m := s.statM; m.any(); m = m.rest() {
		sl := m.first()
		if k.gen.sCls[s.cardAt(sl)]&cls == 0 {
			continue
		}
		lost := s.genLost(k, sl)
		p := &s.perms[sl]
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if lost && !s.keptOnLossAt(k, sl, a) {
				continue // a source that lost its abilities keeps only the effects that began before layer 6
			}
			if a.Kind == kop.AbStatic || a.Kind == kop.AbReplacement {
				if !f(statSrc{a: a, sl: sl, ctrl: p.ctrl, obj: p.obj}) {
					return
				}
			}
		}
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			for j := range e.Abilities {
				a := &e.Abilities[j]
				if a.Kind == kop.AbStatic {
					if !f(statSrc{a: a, sl: NoSlot, ctrl: pp, obj: NoObj, emb: pp<<4 | uint8(i+1)}) {
						return
					}
				}
			}
		}
	}
}

// mayPlayZones is the zones (bit per kop.Zone) from which some
// StatMayPlayFrom static in force, controlled by p, lets p play a card
// (before its condition and filter): priorityMayPlay asks mayPlayStatics
// only about cards in those zones, or in p's graveyard with a permission of
// their own.
func (s *State) mayPlayZones(k *prep, p uint8) uint16 {
	z := uint16(0)
	for m := s.statM; m.any(); m = m.rest() {
		sl := m.first()
		if k.gen.sCls[s.cardAt(sl)]&sclsMayPlay == 0 || s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		mine := s.perms[sl].ctrl == p
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom && a.Static.StFlags&(kop.StFunctionsInGraveyard|kop.StFunctionsInExile) == 0 &&
				(mine || a.Static.StFlags&kop.StSeatActive != 0 && p == s.active) {
				z |= mayPlayZoneBits(&a.Static)
			}
		}
	}
	for i := 0; i < int(s.g.nemb[p]); i++ {
		e := &k.t.Emblems[s.embIdx(p, i)-1]
		for j := range e.Abilities {
			a := &e.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom && a.Static.StFlags&(kop.StFunctionsInGraveyard|kop.StFunctionsInExile) == 0 {
				z |= mayPlayZoneBits(&a.Static)
			}
		}
	}
	return z
}

// grantsLive reports whether grantedAbilitiesN can report a grant for some
// slot: an override record, a global grant in the table, or a static
// grant source on the battlefield (kprio2).
func (s *State) grantsLive(k *prep) bool {
	return k.gen.anyGrant && (s.fxM.any() || k.gen.globGrant || s.grantM.any())
}

// grantMay reports whether grantedAbilitiesN can report a grant for sl
// (a superset: false means it calls f for nothing), so a caller skips the
// walk and its closures for a slot with none (kprio2).
func (s *State) grantMay(k *prep, sl Slot) bool {
	return k.gen.anyGrant && (s.fxM.has(sl) || k.gen.globGrant || s.grantM.any())
}

// sbaPTQuiet reports whether no static in force can change a permanent's
// P/T as ptGen derives it: no emblem, no table-wide layer walk, and no
// battlefield permanent printing a P/T, CDA, override or subtype-grant
// static. Then a creature with no record, no walk flag and no set/copy
// override has at least its printed toughness (sba's fast path, kprio2).
func (s *State) sbaPTQuiet(k *prep) bool {
	if s.g.nemb[0]+s.g.nemb[1] != 0 || k.gen == nil || k.gen.fxNeed&(fxNeedGlobal|fxNeedSubGrant) != 0 {
		return false
	}
	for m := s.statM; m.any(); m = m.rest() {
		if k.gen.sCls[s.cardAt(m.first())]&sclsPTRel != 0 {
			return false
		}
	}
	return true
}
