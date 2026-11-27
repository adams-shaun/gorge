package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An announced Announce$ X alternative cost: the Shoal cycle's
// ExileFromHand<1/Card.Colour+Other+cmcEQX> ("you may exile a red card with
// mana value X from your hand rather than pay this spell's mana cost",
// Blazing and Disrupting Shoal; CR 601.2b, 107.3a). The table encodes it as
// CostExileGrave with ExileHand and Cost.N 0 (a literal ExileFromHand<N/..>
// has N >= 1, so every fixed-count reader stays correct on it; the same
// encoding as the announced ExileFromGrave<X/Spec>, cost_exilex.go, and the
// announced PayLife<X>, cost_paylifex.go). gorge: statics_altcost.go's
// altCostView.announce is read on the chosen alternative cost
// (cast_begin.go's pc.announceX), its xAsk arm (cast_subcounter.go) poses
// one KChoose X over the hand's distinct mana values at which a card still
// matches the exile spec (altCostXCandidates, ascending, no pool bound --
// the alt cost pays no mana), aborting the cast when none does (CR 733.1,
// nothing has moved), and cast_asks.go's exAsk then asks the one pick over
// the hand at the announced X; the offer gate (legal_walk_hand.go) withholds
// while the candidate set is empty. The filter's cmcEQX clause is the
// kernel's PredCMCLEV with PredV X (filters_gen.go): the pick reads the
// announced X through the building entry (evalV), the candidate walk binds
// it per card through the offer probe (probe.x, as sacXPoolOffer sweeps).
// Cold code: the build reaches it only behind genPrep.anyAnnounceX.

// prepAnnounceX sets anyAnnounceX.
func prepAnnounceX(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost && announceHandXCost(&a.Cost) {
				g.anyAnnounceX = true
			}
		}
	}
}

// announceHandXCost reports whether c is the announced X-bound hand exile:
// the hand-exile alternative cost's exile alone with Cost.N 0 (the pick
// takes the one card its filter matches at the announced X).
func announceHandXCost(c *kop.Cost) bool {
	return c != nil && c.Flags == kop.CostExileGrave && c.ExileHand && c.N == 0
}

// announceXBuilding reports whether the entry being built is a cast for its
// own announced X-bound hand-exile alternative cost (the bsX ask's gate,
// cast.go).
func (s *State) announceXBuilding(k *prep) bool {
	g := k.gen
	return g != nil && g.anyAnnounceX && s.bld().altAdd == buildAltCost && announceHandXCost(&s.altAbOf(k).Cost)
}

// announceXMVs returns the distinct mana values at which at least one of
// p's hand cards still matches the announced exile cost's spec (gorge
// altCostXCandidates, statics_altcost.go): bit v is the candidate value v,
// the set the announce ask offers and the offer gate withholds on. Each
// hand card is judged at its own mana value through the offer probe (the
// cmcEQX clause reads X through evalV), the card being cast -- exclude --
// is never its own fodder. A mana value above 63 is no candidate: it is
// unrepresentable in an ask option's byte (maxXAsk is 60) and no printed
// card reaches it.
func (s *State) announceXMVs(k *prep, p uint8, c *kop.Cost, exclude Obj) uint64 {
	mask := uint64(0)
	pl := &s.pl[p]
	sv := s.g.probe
	card := CardID(0)
	if exclude != NoObj {
		card = s.objs[exclude].card
	}
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		if o == exclude || s.picked(o) {
			continue
		}
		cd := s.objs[o].card
		v := s.cardMV(k, cd, o, false)
		if v < 0 || v > 63 || mask&(1<<uint(v)) != 0 {
			continue
		}
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: card, ctrl: p, x: uint8(v + 1)}
		if c.Filter != 0 && !s.matchCardGen(k, c.Filter, cd, s.objs[o].owner(), p, o) {
			continue
		}
		mask |= 1 << uint(v)
	}
	s.g.probe = sv
	return mask
}

// announceXOptions poses the announce ask of the entry being built for its
// announced X-bound hand-exile alternative cost (bsX; gorge xAsk's announce
// arm, cast_subcounter.go): one KChoose X over the candidate values,
// ascending. False when the set is empty -- the hand changed under the
// offer gate, and gorge's abortCast reverses the cast (CR 733.1).
//
//go:noinline
func (s *State) announceXOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	mask := s.announceXMVs(k, e.ctrl, &s.altAbOf(k).Cost, e.obj)
	if mask == 0 {
		if s.ctl == ctlBuild {
			s.reverseCast(k) // gorge "announce cost no longer payable; cast aborted"
		}
		return false
	}
	d.N = 0
	for v := 0; v < 64; v++ {
		if mask&(1<<uint(v)) != 0 {
			d.add(MakeAction(AkChoose, 0, 0, uint8(v)))
		}
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChX, e.ctrl
	return true
}
