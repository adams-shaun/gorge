package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// RetraceGrantStats counts the retrace casts a StatRetraceGrant static gave a
// card without printed Retrace (beginCast).
var RetraceGrantStats struct{ Casts atomic.Int64 }

// A battlefield static granting characteristics to cards in an
// off-battlefield zone. Its kinds are not StatContinuous, so no battlefield
// reader (the layer walks, matchPermGen) sees them: kop.StatOffZoneGrant (a
// subtype or simple keyword grant; featview.go) and kop.StatRetraceGrant (a
// graveyard retrace: Six; Deeproot Historian; Wrenn and Six's emblem). A hand
// miracle grant (Aminatou, Veil Piercer) is kop.StatMiracleGrant (kw_miracle.go).
// The grant is read live from the battlefield, as
// kw_fbgrant.go reads a flashback grant: no mark is stamped on the card and
// no State byte records it.

// offKwStatic is the first StatRetraceGrant static in force that p controls
// whose filter matches card c owned by owner
// in zone z, or nil. The card is read as a card in its zone: the filter's
// Zone is the grant's AffectedZone$ (ZoneNone for All).
func (s *State) offKwStatic(k *prep, p, owner uint8, c CardID, z kop.Zone, o Obj) *kop.Ability {
	if k.gen == nil {
		return nil
	}
	var got *kop.Ability
	s.eachStaticC(k, sclsOffKwG, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatRetraceGrant || st.ctrl != p {
			return true
		}
		if a.Static.Zone != kop.ZoneAny && a.Static.Zone != z {
			return true
		}
		if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, c, owner, p, o, nil) {
			return true
		}
		got = a
		return false
	})
	return got
}

// FeatOffKwNames is the off-zone keywords the off-zone grant statics give
// object o now, in static-walk order: a graveyard card's retrace
// (StatRetraceGrant) and its granted unearth (StatUnearthGrant,
// kw_unearthgrant.go). The adapter's view projection lists them beside the
// printed and KwMask-granted keywords.
func (s *State) FeatOffKwNames(o Obj) []string {
	ob := &s.objs[o]
	k := s.k()
	if k.gen == nil || ob.zone == kop.ZoneBattlefield {
		return nil
	}
	var out []string
	s.eachStaticC(k, sclsOffKwG, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Zone != kop.ZoneAny && a.Static.Zone != ob.zone {
			return true
		}
		if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, ob.card, ob.owner(), st.ctrl, o, nil) {
			return true
		}
		switch a.Static.Kind {
		case kop.StatRetraceGrant:
			out = append(out, "Retrace")
		case kop.StatUnearthGrant:
			out = append(out, "Unearth")
		case kop.StatCyclingGrant:
			out = append(out, "Cycling") // a typecycling grant lists as Cycling too (kw_cyclinggrant.go)
		}
		return true
	})
	return out
}

// offZoneGrantPrep records a table's StatRetraceGrant or StatOffZoneGrant
// static at prep time: a retrace grant turns on the graveyard offer
// (prep.retrace, kw_retrace.go), set here so prepare's own branch count
// stays put.
func (g *genPrep) offZoneGrantPrep(a *kop.Ability, p *prep) {
	if a.Static.Kind == kop.StatRetraceGrant {
		p.retrace = true
	}
}
