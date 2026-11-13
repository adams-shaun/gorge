package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Q, Stacking the Deck: "That nonland card gains 'Exile a card from your hand:
// Create a copy of this card. You may cast the copy. Activate only during your
// turn and only if this card is exiled.'" gorge's Animate Abilities$ registers
// a layer-6 AddAbilities effect on the exiled card (Duration$ Permanent: it
// ends when the card leaves exile, effectMoveSweep), and exileGrantedWalk
// offers the body, an ActivationZone$ Exile ability, to the card's owner.
//
// The kernel records the grant (ExtGrantExile) as a (card, version, host)
// triple in genState.qgO / qgV / qgH, three bytes of each of qgN slots (24
// bytes of State); a record is live while the card is still in exile at the
// recorded version (a zone change bumps it, CR 400.7). The ability is the host
// card's latent ability whose ActZone is Exile (the host stays an object after
// it leaves the battlefield, so the body outlives it as gorge's does), offered
// as AkActivate{source: the exiled card, target: the host, mode: 0x80|ability}
// like any other granted ability, on the owner's own turn (graveActCond reads
// the PlayerTurn$ gate) with a hand card to exile as its cost.

// qgN is the exile grants live at once.
const qgN = 8

// DebugQStackNoGrant switches the grant off (the Q, Stacking the Deck mutation
// test: the lockstep must then diverge, gorge offering the ability).
var DebugQStackNoGrant bool

// prepExileGrant arms the grant walk when some program holds an ExtGrantExile.
func prepExileGrant(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.A.K == kop.VConst && kop.ExtOp(in.A.N) == kop.ExtGrantExile {
					g.anyExileGrant = true
				}
			}
		}
	}
}

// exileGrantLive reports whether record j grants a live ability to card o.
func (s *State) exileGrantLive(j int, o Obj) bool {
	if s.g.qgO[j] != uint8(o)+1 {
		return false
	}
	ob := &s.objs[o]
	return ob.ver == s.g.qgV[j] && ob.zone == kop.ZoneExile
}

// grantExile is ExtGrantExile: every remembered card in exile gains the
// resolving ability's host's exile ability. A card already granted by this
// host stays one grant (gorge registers one effect per resolution, and two
// effects offer the same body twice, which the offer collapses).
//
//go:noinline
func (s *State) grantExile(k *prep, in *kop.Instr) {
	if DebugQStackNoGrant {
		return
	}
	host := s.res.e.obj
	if host == NoObj {
		return
	}
	for i := 0; i < int(s.g.nrem); i++ {
		o := s.g.rem[i]
		if s.objs[o].zone != kop.ZoneExile {
			continue
		}
		free := -1
		for j := 0; j < qgN; j++ {
			if s.exileGrantLive(j, o) && s.g.qgH[j] == uint8(host)+1 {
				free = -2
				break
			}
			if free == -1 && (s.g.qgO[j] == 0 || !s.exileGrantLive(j, Obj(s.g.qgO[j]-1))) {
				free = j
			}
		}
		if free == -2 {
			continue
		}
		if free < 0 {
			s.capFull(StallExileGrant)
			return
		}
		s.g.qgO[free], s.g.qgV[free], s.g.qgH[free] = uint8(o)+1, s.objs[o].ver, uint8(host)+1
	}
}

// exileGrantAb is the index of the host card's exile-granted ability (its one
// latent activated ability that works from exile), -1 when it has none.
func exileGrantAb(k *prep, c CardID) int {
	abs := k.t.Cards[c].Abilities
	for i := range abs {
		if a := &abs[i]; a.Kind == kop.AbActivated && a.ActZone == kop.ZoneExile && a.AFlags&kop.AfLatent != 0 {
			return i
		}
	}
	return -1
}

// priorityExileGrant offers p the abilities their exiled cards were granted
// (gorge exileGrantedWalk): in exile order, the owner's turn only, a hand
// card to pay the cost.
//
//go:noinline
func (s *State) priorityExileGrant(k *prep, d *Decision, p uint8) {
	pl := &s.pl[p]
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		for j := 0; j < qgN; j++ {
			if !s.exileGrantLive(j, o) {
				continue
			}
			host := Obj(s.g.qgH[j] - 1)
			// a host on the battlefield is read as it is now (a Dominion
			// Saboteur that entered as Q, Stacking the Deck grants Q's body,
			// CR 707.2); one that left keeps its printed card, as activateGen
			hc := s.objs[host].card
			if s.objs[host].zone == kop.ZoneBattlefield {
				hc = s.cardAt(s.objs[host].slot)
			}
			ai := exileGrantAb(k, hc)
			if ai < 0 {
				continue
			}
			a := &k.t.Cards[hc].Abilities[ai]
			if !s.graveActCond(k, a, p, o) || !s.costObjPayable(k, p, &a.Cost, o, NoObj) {
				continue
			}
			act := MakeAction(AkActivate, TCode(o), TCode(host), 0x80|uint8(ai))
			dup := false
			for x := 0; x < int(d.N); x++ {
				dup = dup || d.Options[x] == act
			}
			if !dup {
				d.add(act)
			}
		}
	}
}
func init() { stallNames[StallExileGrant] = "exilegrant" }

// spellRequiresTarget reports whether card c's plain cast has a target slot it
// must fill at announcement (requiredTarget for a cast not yet begun).
func spellRequiresTarget(k *prep, c CardID) bool {
	ab := spellAb(k, c)
	for ti := 0; ti < kop.MaxTargets; ti++ {
		t := tspec(k, c, ab, ti)
		if t == nil || t.Kind == kop.TgtNone || t.UpTo || t.TFlags&kop.TgfAtResolution != 0 {
			continue
		}
		if t.TFlags&kop.TgfBoundsV != 0 && t.MinV.K != kop.VConst {
			continue // "X target creatures": X is announced first (CR 601.2b) and may be 0, so the ask is not yet required
		}
		if ab != 255 {
			if tm := k.t.Cards[c].Abilities[ab].TgtModes[ti]; tm != 0 {
				continue // a mode-gated slot: the copy cast announces no mode
			}
		}
		return true
	}
	return false
}

// copyCastAbortsBeforeWindow reports whether the paid copy cast of c cannot be
// paid and has a required target: gorge's target menu withholds every
// candidate of an unpayable cast (affordableTargetCandidates), so the cast
// aborts at the target ask and no mana window opens, tapping nothing. Without
// a required target gorge reaches the window, taps what it offers and reverses
// the cast with the taps and their mana standing (loweredWindowOK, CR 733.1).
// An {X} spell is not aborted here: X is announced first (CR 601.2b), with the
// copy on the stack, and copyCastXTargetAbort aborts at the target ask.
func (s *State) copyCastAbortsBeforeWindow(k *prep, c CardID, p uint8, o Obj) bool {
	if !spellRequiresTarget(k, c) || k.t.Cards[c].Cost.X != 0 {
		return false
	}
	if s.copyCensusOpen(k, &s.bld().e) {
		// the window census cannot price a source the payer still has, so
		// gorge's target menu keeps every candidate however short the pool
		// is (affordableTargetCandidates' census fallback, CR 601.2c,
		// 601.2g): the cast raises its menu and reverses in the window.
		return false
	}
	return s.copyCastUnpayable(k, c, p, o, s.restrOriginOfObj(o), 0)
}

// copyCastUnpayable reports whether the paid copy cast of c by p, as object o
// and cast from og, at announced X x, cannot be covered by what p can produce
// (the cast offer's own affordability read).
func (s *State) copyCastUnpayable(k *prep, c CardID, p uint8, o Obj, og restrOrigin, x int) bool {
	var av avail
	s.availFor(k, p, &av)
	base := k.cards[c].cost
	av.delve = s.tbDelveCredit(k, p, c, o, false) // the faces' credit (CR 702.66a); the copy object is a stack spell, not a hand cast
	cost := s.adjustFit(k, &av, p, c, o, base, true)
	if !av.tbLeafCredited(&base) {
		cost = s.delveOfferCredit(k, p, c, o, false, cost) // gorge's choke point credits the generic at this read too
	}
	cost = cost.AtX(x)
	return !s.castAffordsR(k, p, c, og, &cost, &av)
}

// copyCastXTargetAbort: the target ask of a paid Play's copy cast that X, now
// announced, leaves unpayable: gorge's menu withholds every candidate
// (affordableTargetCandidates), so a required target aborts the cast
// (CR 601.2c, 733.1; copyCastAbortsBeforeWindow for the spells without X).
func (s *State) copyCastXTargetAbort(k *prep, t *kop.TargetSpec) bool {
	b := s.bld()
	if k.gen == nil || !k.gen.anyCopyCast || b.e.kind != seSpell || b.bfl&bflLowered == 0 || b.origin != kop.ZoneLibrary ||
		t.UpTo || t.TFlags&kop.TgfBoundsV != 0 || k.t.Cards[b.e.card].Cost.X == 0 {
		return false
	}
	if s.copyCensusOpen(k, &b.e) {
		return false // a census gap keeps the candidate on the menu (affordableTargetCandidates)
	}
	return s.copyCastUnpayable(k, b.e.card, b.e.ctrl, b.e.obj, s.restrOriginOfObj(b.e.obj), int(b.e.x))
}
