package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A Continuous static's GainsAbilitiesOf$ (CR 613.1f, "has all activated
// abilities of ..."; kop.Static.GainsOf, kgen lower_gainsof.go): the static's
// source, its only recipient (AffectedDefined$ Self), may activate each
// activated ability of every battlefield permanent (or graveyard card, Zone
// ZoneGraveyard) the filter matches right now. The walk is stateless: the filter picks the foreign permanents again at
// every priority (gorge layers.go gainedFacesForSpec rescans, legal_granted.go
// grantedAbilitiesLimited lists their faces' AB lines).
//
// GainsAbilitiesOfZones$ Graveyard (kop.Static.Zone = ZoneGraveyard) reads
// the graveyard cards instead: every seat's graveyard in seat order, each in
// its stored order. ZoneExile reads every seat's exile zone the same way.
//
// A gained activation is AkActivate{source: the recipient, target: the
// foreign permanent or graveyard card, mode: 0xC0 | the ability's index in the foreign card}
// (ab < 64). activateGen puts it on the stack as the recipient's ability with
// the foreign card's body (a cost's Self, and the effect's, is the recipient).

// StallGainsOf: a gained ability the walk does not model (a mana ability, an
// activation limit, once each turn, a loyalty ability, one that works from
// another zone, or an index past 63).
const StallGainsOf = 182

func init() { stallNames[StallGainsOf] = "gainsof" }

// gainedMode is the AkActivate mode bit pair of a gained activation.
const gainedMode uint8 = 0xC0

// isGainedMode reports whether an AkActivate mode is a gained activation
// (0xC0|ab), which every reader of the granted bit (0x80) tests first.
func isGainedMode(mode uint8) bool { return mode&gainedMode == gainedMode }

// DebugNoGainsOf is the mutation: the walk offers nothing (the lockstep's
// TestGainsAbilitiesOfMutation must then diverge).
var DebugNoGainsOf bool

// useGainsOf arms the walk (genPrep.uses) for a table with a
// GainsAbilitiesOf$ static, and gives such a table a genPrep.
var useGainsOf = newGenUse(tableHasGainsOf)

// tableHasGainsOf reports whether a card of t has a Continuous static with a
// GainsOf filter.
func tableHasGainsOf(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.GainsOf != 0 {
				return true
			}
		}
	}
	return false
}

// gainedAbilities offers p's gained activations: each GainsOf static p
// controls whose source keeps its abilities and whose condition holds, in the
// static walk's order (contGrants'), each foreign permanent in battlefield
// order, each of its activated abilities in card order. It offers nothing
// unless the table has a GainsOf static (useGainsOf).
func (s *State) gainedAbilities(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	if DebugNoGainsOf || !k.gen.uses.has(useGainsOf) {
		return
	}
	var order [MaxPerm]Slot
	n := s.listenerOrder(s.statM, &order)
	for oi := 0; oi < n; oi++ {
		src := order[oi]
		if s.perms[src].ctrl != p || s.genLost(k, src) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.GainsOf == 0 {
				continue
			}
			st := statSrc{a: a, sl: src, ctrl: p, obj: s.perms[src].obj}
			if !s.statCond(k, st) {
				continue
			}
			if !s.gainedFrom(k, d, st, sorc, av) {
				return
			}
		}
	}
}

// gainedFrom offers the abilities static st's recipient (its source) gains
// from each foreign permanent its filter matches (or, for a static whose Zone
// is the graveyard, from each graveyard card it matches); false when the walk
// stalled.
func (s *State) gainedFrom(k *prep, d *Decision, st statSrc, sorc bool, av *avail) bool {
	first := int(d.N)
	if st.a.Static.Zone == kop.ZoneExile {
		// GainsAbilitiesOfZones$ Exile: every seat's exile zone in seat order,
		// each in its stored order. The filter reads the static's source
		// (ExiledWithSource through exwFlag) and controller; the owner is the
		// exile zone's seat. A face-down card is never exiled by the carriers
		// (Dark Impostor, Patchwork Crawler).
		ctx := evalCtx{src: st.obj, srcSl: st.sl, ctrl: st.ctrl}
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.out(p) {
				continue
			}
			pl := &s.pl[p]
			for i := 0; i < int(pl.exileN); i++ {
				o := pl.exile[i]
				card := s.objs[o].card
				if !s.matchCardGenAt(k, st.a.Static.GainsOf, card, p, st.ctrl, o, &ctx) {
					continue
				}
				if !s.gainedOffer(k, d, st, first, o, &k.t.Cards[card], sorc, av) {
					return false
				}
			}
		}
		return true
	}
	if st.a.Static.Zone == kop.ZoneGraveyard {
		// GainsAbilitiesOfZones$ Graveyard: every seat's graveyard in seat
		// order, each in its stored order. The filter reads the static's
		// source and controller (YouOwn, Other), the owner is the graveyard's
		// seat.
		ctx := evalCtx{src: st.obj, srcSl: st.sl, ctrl: st.ctrl}
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.out(p) {
				continue
			}
			pl := &s.pl[p]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				card := s.objs[o].card
				if !s.matchCardGenAt(k, st.a.Static.GainsOf, card, p, st.ctrl, o, &ctx) {
					continue
				}
				if !s.gainedOffer(k, d, st, first, o, &k.t.Cards[card], sorc, av) {
					return false
				}
			}
		}
		return true
	}
	var order [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &order)
	for oi := 0; oi < n; oi++ {
		fs := order[oi]
		if !s.matchPermGen(k, st.a.Static.GainsOf, fs, st.ctrl, st.sl, layerKw) {
			continue
		}
		if !s.gainedOffer(k, d, st, first, s.perms[fs].obj, &k.t.Cards[s.cardAt(fs)], sorc, av) {
			return false
		}
	}
	return true
}

// gainedOffer offers the recipient of st each activated ability of the
// foreign object fo (card cd); false when the walk stalled.
func (s *State) gainedOffer(k *prep, d *Decision, st statSrc, first int, fo Obj, cd *kop.CardDef, sorc bool, av *avail) bool {
	abs := cd.Abilities
	for j := range abs {
		a := &abs[j]
		if a.Kind != kop.AbActivated && a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if !gainedModelled(a, j) {
			s.capFull(StallGainsOf)
			return false
		}
		if s.cfg&cfgGorgeUnpriced != 0 && !s.poolPays(st.ctrl, &a.Cost.Mana) {
			// gorge's granted sweep prices a gained ability against the
			// real pool only (rules.offerCastable with no hypothetical
			// bound), as the Equipment-granted path in priorityGen
			continue
		}
		if !s.canActivate(k, st.sl, 255, a, sorc, av) {
			continue
		}
		act := MakeAction(AkActivate, TCode(st.obj), TCode(fo), gainedMode|uint8(j))
		if !optionListed(d, first, act) {
			d.add(act)
		}
	}
	return true
}

// gainedModelled reports whether the walk models gained ability j: an
// activated (non-mana) battlefield ability with no activation limit, no
// once-each-turn or loyalty cost, no exhaust record, no host cost, at an
// index the mode's six bits hold.
func gainedModelled(a *kop.Ability, j int) bool {
	const limits = kop.CostOncePerGame | kop.CostOncePerTurn | kop.CostLoyalty
	return a.Kind == kop.AbActivated && j < 64 && a.ActZone == kop.ZoneNone &&
		a.Cost.Flags&limits == 0 && !a.Exhaust && a.Cost.Host == 0
}

// optionListed reports whether act is already among d's options from first on.
func optionListed(d *Decision, first int, act Action) bool {
	for i := first; i < int(d.N); i++ {
		if d.Options[i] == act {
			return true
		}
	}
	return false
}
