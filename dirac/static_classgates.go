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
	sclsEscapeG                    // StatEscapeGrant (kw_escape.go)
	sclsGyCont                     // the card's own StatContinuous that works from its graveyard (static_gycont.go)
	sclsOffKwG                     // StatOffZoneGrant, StatRetraceGrant, StatUnearthGrant, StatCyclingGrant or StatMiracleGrant, an off-zone grant (kw_offkwgrant.go)
	sclsNonObj                     // a static that can change a characteristic and may read what is not an object's own fields (static_sbaskip.go)
	sclsTapCmd                     // a command-zone StatTapExtra on a seated Vanguard (Sisay; mana_tapextra.go eachTapExtra)
)

// prepOverrideLandwalk (called from prepStaticClasses, once per table; kw_evasion.go landwalkPumped reads it) sets genPrep.anyLandwalk when any override record of
// the table grants landwalk (a Pump KW$ Landwalk:<spec>, lower_landwalkgrant).
func prepOverrideLandwalk(t *kop.Table, g *genPrep) {
	for i := range t.Overrides {
		if t.Overrides[i].Landwalk != 0 {
			g.anyLandwalk = true
		}
	}
}

// prepStaticClasses fills g.sCls and returns whether any card carries a
// command-zone StatTapExtra static (a Vanguard's; mana_tapextra.go eachTapExtra).
func prepStaticClasses(t *kop.Table, g *genPrep) (anyCmdTap bool) {
	prepOverrideLandwalk(t, g)
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
			if !staticObjectOnly(t, sa) {
				m |= sclsNonObj
			}
			switch sa.Kind {
			case kop.StatContinuous:
				if sa.StFlags&kop.StFunctionsInGraveyard != 0 {
					m |= sclsGyCont
				}
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
				// a two-zone self grant (Squee, the Immortal's
				// Exile,Graveyard with EffectZone$ Exile,Graveyard) carries
				// BOTH function flags on one static, so the card's own
				// permission is offered from either zone: both class gates
				// set (the self walk keys on the flags)
				if sa.StFlags&kop.StFunctionsInGraveyard != 0 {
					m |= sclsGyMay
				}
				if sa.StFlags&kop.StFunctionsInExile != 0 {
					m |= sclsExMay
				}
				if sa.StFlags&(kop.StFunctionsInGraveyard|kop.StFunctionsInExile) == 0 {
					m |= sclsMayPlay
				}
			case kop.StatTapExtra:
				if sa.Zone == kop.ZoneCommand && c.Types&kop.TVanguard != 0 {
					// a Vanguard's printed static (Sisay): in force from the
					// command zone, walked by eachTapExtra, never by eachStatic
					m |= sclsTapCmd
					anyCmdTap = true
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
			case kop.StatEscapeGrant:
				m |= sclsEscapeG
			case kop.StatOffZoneGrant, kop.StatRetraceGrant, kop.StatUnearthGrant, kop.StatCyclingGrant, kop.StatMiracleGrant:
				m |= sclsOffKwG
			}
		}
		g.sCls[i] = m
	}
	return anyCmdTap
}

// eachStaticC is eachStatic over the permanents whose card prints a static
// of a class in cls (emblems all walked): f sees exactly the statics of
// those classes eachStatic would show it, in the same order. It leaves out
// the graveyard Continuous statics (static_gycont.go), as eachStatic does:
// those are plain keyword and landwalk grants, read by the layer-6 walk
// (prepL6 turns it on for them) and by the readers that call eachGyStatic.
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
	return k.gen.uses.has(useGainedMana) || k.gen.anyGrant && (s.fxM.any() || k.gen.globGrant || s.grantM.any())
}

// grantMay reports whether grantedAbilitiesN can report a grant for sl
// (a superset: false means it calls f for nothing), so a caller skips the
// walk and its closures for a slot with none (kprio2).
func (s *State) grantMay(k *prep, sl Slot) bool {
	return k.gen.uses.has(useGainedMana) || k.gen.anyGrant && (s.fxM.has(sl) || k.gen.globGrant || s.grantM.any())
}

// sbaPTQuiet reports whether no static in force can change a permanent's
// P/T as ptGen derives it: no emblem, no table-wide layer walk, and no
// battlefield permanent printing a P/T, CDA, override or subtype-grant
// static. Then a creature with no record, no walk flag and no set/copy
// override has at least its printed toughness (sba's fast path, kprio2).
func (s *State) sbaPTQuiet(k *prep) bool {
	if s.anyEmb() || k.gen == nil || k.gen.fxNeed&(fxNeedGlobal|fxNeedSubGrant) != 0 {
		return false
	}
	for m := s.statM; m.any(); m = m.rest() {
		if k.gen.sCls[s.cardAt(m.first())]&sclsPTRel != 0 {
			return false
		}
	}
	return true
}

// staticObjectOnly reports whether sa's effect on a permanent's
// characteristics reads only objects' own fields: a plain Continuous static
// (constant amounts, no condition, no grant, no flag) over a filter of
// controller, ownership, type, colour, subtype and the tapped, attacking,
// token, self and attached tests; or a kind that changes no characteristic.
// False for every other kind and filter (a dynamic amount, a condition, a
// CDA, an override, a control change, a predicate program, a turn tally).
func staticObjectOnly(t *kop.Table, sa *kop.Static) bool {
	switch sa.Kind {
	case kop.StatReduceCost, kop.StatIncreaseCost, kop.StatMayPlayFrom, kop.StatReduceAbility, kop.StatFbGrant, kop.StatWarpGrant,
		kop.StatEscapeGrant, kop.StatOffZoneGrant, kop.StatRetraceGrant, kop.StatCantBlock, kop.StatCantBlockBy, kop.StatCantAttack,
		kop.StatMustAttack, kop.StatCastWithFlash, kop.StatAlternativeCost, kop.StatNoMaxHandSize, kop.StatExtraLandPlay, kop.StatCantBeCast:
		return true
	case kop.StatContinuous:
	default:
		return false
	}
	if sa.StFlags != 0 || sa.VA != (kop.ValueRef{}) || sa.VB != (kop.ValueRef{}) || sa.Cond2 != 0 || sa.Cond != 0 || sa.OvGate != 0 ||
		sa.Grant != 0 || sa.GrantSt != 0 || sa.AddChosen || sa.Override != 0 {
		return false
	}
	const plain = kop.FOther | kop.FNontoken | kop.FToken | kop.FTapped | kop.FUntapped | kop.FAttacking | kop.FNotAttacking |
		kop.FSelf | kop.FAttachedBySrc | kop.FNotAttachedBySrc
	for f := sa.Filter; f != 0; f = t.Filters[f].Or {
		fl := &t.Filters[f]
		if fl.CtrlTgt != 0 || fl.Flags&^plain != 0 || fl.Flags2 != 0 || fl.Pred != kop.PredNone || fl.Ctrl > kop.CtrlOpp || fl.Owner > kop.CtrlOpp {
			return false
		}
	}
	return true
}
