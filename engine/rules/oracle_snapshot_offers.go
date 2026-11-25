package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// oracleSnapshotOffers reads the already-filtered priority actions PLUS the
// potential-action walk's plays, deduped by (source, kind, label). The
// priority offer's gate (castable, and the mana-ability payability read)
// prices against the seat's floating pool alone -- the tap-first decision
// model the bots, the wire and the dirac lockstep replay on, which must not
// change -- while CR 601.2g-h and 602.2b make a play legal when its cost can
// be paid by activating mana abilities during the announcement. The
// snapshot's `offered` is that CR-legal set (the side Forge and XMage report;
// xproof compares [offered]), so the potential-action walk -- the engine's
// own legal-offer walk priced against the untapped-source bound
// (PotentialMana): same timing, restriction, target and non-mana gates, the
// mana part over-bound, every non-mana part against the real state -- fills
// in what the floating-pool gate withheld. Generic mana actions expand
// through the same named legal-ability reader the oracle assertion uses; the
// labels keep that reader's spelling. This must not implement another
// legality walk.
func oracleSnapshotOffers(r *oracleRun, d *decision.Decision) []OracleSnapOffered {
	var out []OracleSnapOffered
	seen := make(map[OracleSnapOffered]bool, len(d.Options))
	add := func(row OracleSnapOffered) {
		if seen[row] {
			return
		}
		seen[row] = true
		out = append(out, row)
	}
	for _, o := range d.Options {
		if o.Obj == 0 {
			continue
		}
		source := r.objRef(r.e.G.Obj(o.Obj))
		if oracleOpCodes.Code(o.Kind) == oracleOpActivate {
			for _, label := range r.manaAbilityLabels(d.Player, o.Obj) {
				add(OracleSnapOffered{Source: source, Kind: "activate", Label: label})
			}
			continue
		}
		kind := o.Kind
		switch oracleOpCodes.Code(kind) {
		case oracleOpCast, oracleOpPlay:
		default:
			// The priority walk's existing real-play classification includes
			// printed, granted and keyword/special actions. Don't grow a
			// parallel list here when another activation kind is added.
			if !potentialPlayKind(kind) {
				continue
			}
			kind = "activate"
			if o.Kind == "play_land" {
				kind = "play"
			}
		}
		add(OracleSnapOffered{Source: source, Kind: kind, Label: o.Label})
	}
	r.addPotentialOffers(d, add)
	return out
}

// addPotentialOffers appends the potential-action walk's plays and the mana
// activations its bound pays for, into add (already deduped). The walk is
// the engine's own (potentialWalkOf, PotentialMana for the bound); its
// non-mana plays need no extra read here. The mana labels do: the walk's
// mana section gates every activation on the floating pool -- the priority
// offer's own read, which is what withheld e.g. a Station 12+ {G},{T} ability
// from an empty pool -- so the same choke-point membership and the same
// payability gate are re-run against the bound the casts were priced with.
// Both halves are pure reads; the map in the caller is only indexed, never
// ranged.
func (r *oracleRun) addPotentialOffers(d *decision.Decision, add func(OracleSnapOffered)) {
	if r.e.G.Over {
		return
	}
	p := d.Player
	hyp, opts := r.e.potentialWalkOf(p, true)
	for _, o := range opts {
		if !potentialPlayKind(o.Kind) {
			continue
		}
		kind := o.Kind
		switch oracleOpCodes.Code(kind) {
		case oracleOpCast, oracleOpPlay:
		default:
			// Printed, granted and keyword/special plays activate, the same
			// classification the priority options get.
			kind = "activate"
			if o.Kind == "play_land" {
				kind = "play"
			}
		}
		add(OracleSnapOffered{Source: r.objRef(r.e.G.Obj(o.Obj)), Kind: kind, Label: o.Label})
	}
	for _, id := range r.potentialManaSources(p) {
		for _, label := range r.potentialManaLabels(p, id, &hyp) {
			add(OracleSnapOffered{Source: r.objRef(r.e.G.Obj(id)), Kind: "activate", Label: label})
		}
	}
}

// potentialManaSources walks the zones the legal-offer walk's mana section
// walks, in zone order: the battlefield for every seat (an ability another
// player's Activator$ permit reaches p's offer) plus p's hand and graveyard
// (Spirit Guides and Jack-o'-Lantern, CR 605.2a). The choke point behind the
// labels re-checks zone, controller and every other non-payability gate, so
// a generous zone list only costs reads.
func (r *oracleRun) potentialManaSources(p state.PlayerID) []state.ObjID {
	var ids []state.ObjID
	for seat := range r.e.G.Players {
		ids = append(ids, r.e.G.Zone(state.ZBattlefield, state.PlayerID(seat))...)
	}
	ids = append(ids, r.e.G.Zone(state.ZHand, p)...)
	ids = append(ids, r.e.G.Zone(state.ZGraveyard, p)...)
	return ids
}

// potentialManaLabels is manaAbilityLabels with the payability gate priced
// against the potential bound hyp, so an ability the floating pool cannot
// pay but the untapped sources can (CR 602.2b) is reported beside the ones
// the priority offer already lists. Printed and intrinsic members come from
// the choke point with its live-pool gate deferred
// (appendAvailableManaAbilitiesGate, the same membership PotentialMana
// records), each then through the walk's own payability read -- the priced
// cost (pricedManaCC) through manaCostPayable, whose mana half prices
// against hyp and whose every other read stays against the real state.
// Granted mana members (an AddAbilities grant's AB$ Mana) are read from
// grantedAbilities under the same non-payability gates the choke point's
// granted block applies, their payability through the same
// manaAbilityPayablePool the cast-window probe prices with. Both halves
// label through the same ManaAbilityLabel manaAbilityLabels uses, so an
// ability the floating pool already pays dedupes against the priority
// offer's row instead of duplicating it. A granted ManaReflected member is
// left to the choke point's own read (its candidate walk is a separate
// surface).
func (r *oracleRun) potentialManaLabels(seat state.PlayerID, id state.ObjID, hyp *state.Mana) []string {
	var labels []string
	o := r.e.G.Obj(id)
	for _, ma := range r.e.appendAvailableManaAbilitiesGate(nil, nil, seat, id, true) {
		cc := r.e.abilityCompiledCost(ma)
		if mf := r.e.manaFactsOf(ma); mf != nil {
			cc = mf.cost
		}
		cc = r.e.pricedManaCC(seat, id, ma, cc)
		if !r.e.manaCostPayable(seat, o, id, cc, hyp) {
			continue
		}
		labels = append(labels, pay.ManaAbilityLabel(ma, pay.ChosenProducedColour(r.e.G, id)))
	}
	for _, ga := range r.e.grantedAbilities(seat, id) {
		ma := ga.SA
		if ma == nil || ma.API == "ManaReflected" || !cards.IsManaAbilitySA(ma) || r.e.isLoyaltyAbility(ma) {
			continue
		}
		if !abilityZoneOK(ma, o.Zone) || !r.e.activatorAllows(seat, id, ma) || r.e.abilityRestricted(seat, id, ma) ||
			!pay.ManaActivationGateHolds(asPayer(r.e), seat, id, ma) || !pay.ManaSVarGateOK(asPayer(r.e), o, seat, id, ma) {
			continue
		}
		if !r.e.manaAbilityPayablePool(seat, id, ma, hyp) {
			continue
		}
		labels = append(labels, pay.ManaAbilityLabel(ma, pay.ChosenProducedColour(r.e.G, id)))
	}
	return labels
}
