package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A printed S:Mode$ CantSacrifice with neither ValidCause$ nor ForCost$
// (kop.StatCantSacrifice; CR 701.21a; gorge rules/layers_restrict.go
// sacrificeBlocked): no permanent matching its Filter can be sacrificed,
// whatever asks for it. gorge consults the restriction at every sacrifice
// candidate choke point, and so does the kernel:
//   - a cost: countSac and the pick candidates (sacMatch), a self-sacrifice
//     cost in canActivate and the unless window (PaySacSelf), and a
//     sacrificing mana ability, which actForbidden makes no mana source;
//   - an effect: the each-player and forced sacrifice walks, SacrificeAll,
//     a delayed "sacrifice it" and the Saga's state-based sacrifice.
// Quark, Humble Proprietor ("You can't sacrifice Treasure tokens") is the
// Standard carrier: his controller's Treasures add no mana.

// DebugCantSacOff (test only, TestCantSacMutation): sacBlocked never blocks,
// so a Treasure gorge keeps is sacrificed here and the lockstep diverges.
var DebugCantSacOff bool

// prepCantSac arms genPrep.anyCantSac when some card has a StatCantSacrifice.
func prepCantSac(t *kop.Table, g *genPrep) {
	g.anyCantSac = tableHasStatic(t, kop.StatCantSacrifice)
}

// sacBlocked reports whether a StatCantSacrifice forbids sacrificing the
// permanent at sl. It inlines to one flag test when no card in the table has
// the static.
func (s *State) sacBlocked(k *prep, sl Slot) bool {
	return k.gen != nil && k.gen.anyCantSac && s.sacBlockedBy(k, sl)
}

// sacBlockedBy is sacBlocked's walk. The static's Filter is read with its own
// controller as "you", like gorge's specCtx(source, controller); eachStatic
// skips a source that lost its abilities, as gorge's activeStatics does.
//
//go:noinline
func (s *State) sacBlockedBy(k *prep, sl Slot) bool {
	if DebugCantSacOff {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantSacrifice {
			return true
		}
		if s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// sacMatch is matchPerm for a sacrifice candidate: a blocked permanent is
// never offered, counted or taken.
func (s *State) sacMatch(k *prep, f kop.FilterID, sl Slot, p uint8, src Slot) bool {
	return s.matchPerm(k, f, sl, p, src) && !s.sacBlocked(k, sl)
}

// sacMatchGen is sacMatch on matchPermGen's full layer view.
func (s *State) sacMatchGen(k *prep, f kop.FilterID, sl Slot, p uint8, src Slot) bool {
	return s.matchPermGen(k, f, sl, p, src, layerFull) && !s.sacBlocked(k, sl)
}

// manaSacBlocked reports whether the planner's mana ability of the
// permanent at sl sacrifices it and it can't be sacrificed: the cost can't
// be paid (CR 602.2b, 601.2h), so gorge never activates it.
func (s *State) manaSacBlocked(k *prep, sl Slot) bool {
	pc := &k.cards[s.cardAt(sl)]
	if !pc.manaSac && !pc.manaSacOnly {
		if pc.manaAb < 0 {
			return false
		}
		if a := &k.t.Cards[s.cardAt(sl)].Abilities[pc.manaAb]; a.Cost.Flags&kop.CostSacSelf == 0 {
			return false
		}
	}
	return s.sacBlocked(k, sl)
}
