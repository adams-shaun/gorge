package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// Beam me up: "you may cast this card from your graveyard for [cost] if you also
// return a creature you control to its owner's hand. Then exile this spell." It
// is flashback's shape with one more cost part, so it shares flashback's offer
// walk and its exile on leaving the stack (state.ExilesLeavingStack).
//
// A grant with no cost of its own costs the card's mana cost (Scotty, Miracle
// Worker). Each distinct cost is its own permission, as with flashback, and the
// two kinds differ in what may be returned: S:Mode$ CantBeBeamedUp's
// ValidSA$ Spell.KeywordSource bars a creature only from paying for a keyword it
// grants itself, so "Scotty can't be beamed up to cast a spell this way" leaves
// him returnable for a spell's printed beam me up.

func init() { effects.RegisterNonAPI("kw:Beam me up", "stat:CantBeBeamedUp") }

const (
	beamMeUpMode = "beam_me_up"
	// beamReturnSpec and beamReturnGrantedSpec are the Return cost part's specs,
	// answered by matchesBeamReturn rather than the filter grammar because they
	// read the statics and the spell being cast.
	beamReturnSpec        = "BeamMeUpCreature"
	beamReturnGrantedSpec = "BeamMeUpCreatureGranted"
)

// beamMeUpInstance is one beam me up permission on a card.
type beamMeUpInstance struct {
	cost    string
	granted bool
}

// beamMeUpInstances lists id's distinct beam me up permissions, the printed one
// first. A card with no mana cost has no payable grant.
func (e *Engine) beamMeUpInstances(id state.ObjID) []beamMeUpInstance {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || !e.hasKeywordH(id, kwhBeamMeUp) {
		return nil
	}
	f := o.Face()
	var out []beamMeUpInstance
	printed, hasPrinted := f.KeywordCostParam(kwhBeamMeUp.S)
	if hasPrinted {
		out = append(out, beamMeUpInstance{cost: printed})
	}
	for _, k := range e.Derived(id).Keywords {
		if cardsKeywordHead(k) != kwhBeamMeUp.S {
			continue
		}
		raw := f.ManaCost
		if len(k) > len(kwhBeamMeUp.S) {
			raw = cards.KeywordCostField(k[len(kwhBeamMeUp.S)+1:])
		}
		if costFieldsEqual(raw, "") {
			continue
		}
		dup := false
		for _, have := range out {
			dup = dup || costFieldsEqual(raw, have.cost)
		}
		if !dup {
			out = append(out, beamMeUpInstance{cost: raw, granted: true})
		}
	}
	return out
}

// beamMeUpCost is one permission's cost: its mana and the returned creature.
func beamMeUpCost(in beamMeUpInstance) Cost {
	c := ParseCost(in.cost)
	spec := beamReturnSpec
	if in.granted {
		spec = beamReturnGrantedSpec
	}
	c.Return = append(c.Return, CostPart{N: 1, Spec: spec, Desc: "creature"})
	return c
}

// beamMeUpCostFor is the cost a cast option charges: the permission it names by
// Option.Cost, else the card's first.
func (e *Engine) beamMeUpCostFor(id state.ObjID, opt decision.Option) Cost {
	instances := e.beamMeUpInstances(id)
	for _, in := range instances {
		if opt.Cost != "" && costFieldsEqual(in.cost, opt.Cost) {
			return beamMeUpCost(in)
		}
	}
	if len(instances) > 0 {
		return beamMeUpCost(instances[0])
	}
	return Cost{}
}

// matchesBeamReturn answers a beam me up Return part: a creature you control
// that no CantBeBeamedUp static bars from paying for spell.
func (e *Engine) matchesBeamReturn(spec string, id state.ObjID, you state.PlayerID, spell state.ObjID) bool {
	if !e.matchesSpecFrom("Creature.YouCtrl", id, you, spell) {
		return false
	}
	for _, sv := range e.activeStatics("CantBeBeamedUp") {
		if !e.continuousGateHolds(sv) {
			continue
		}
		if vc := sv.ParamStr(cards.PKValidCard); vc != "" && !e.matchesSpecFrom(vc, id, sv.Controller, sv.Source) {
			continue
		}
		if sv.ParamStr(cards.PKValidSA) == "Spell.KeywordSource" &&
			(spec != beamReturnGrantedSpec || !e.onlyBeamMeUpGrantor(sv.Source, spell)) {
			continue
		}
		return false
	}
	return true
}

// onlyBeamMeUpGrantor reports whether source is the one permanent granting spell
// beam me up. With two grantors the cast can name the other's grant, so neither
// is barred.
func (e *Engine) onlyBeamMeUpGrantor(source, spell state.ObjID) bool {
	found := false
	for _, sv := range e.activeStatics("Continuous") {
		grants := false
		for _, kw := range cards.SplitKeywordList(sv.ParamStr(cards.PKAddKeyword)) {
			grants = grants || cardsKeywordHead(kw) == kwhBeamMeUp.S
		}
		if !grants || !e.continuousGateHolds(sv) ||
			!e.matchesSpecFrom(sv.ParamStr(cards.PKAffected), spell, sv.Controller, sv.Source) {
			continue
		}
		if sv.Source != source {
			return false
		}
		found = true
	}
	return found
}

// offerBeamMeUp appends id's beam me up casts to out.
func (w *legalWalk) offerBeamMeUp(p state.PlayerID, id state.ObjID, f *cards.Face, out *[]decision.Option) {
	for i, in := range w.e.beamMeUpInstances(id) {
		c := beamMeUpCost(in)
		if !w.offerCastable(p, id, pay.WithSpellAbilityExtras(f, c), spellScope(beamMeUpMode), false) {
			continue
		}
		opt := decision.Option{Index: len(*out), Kind: "cast", Label: "Cast " + f.Name + " (beam me up)", Obj: id, Mode: beamMeUpMode}
		if i > 0 {
			opt.Label = "Cast " + f.Name + " (beam me up " + in.cost + ")"
			opt.Cost = in.cost
		}
		*out = append(*out, opt)
	}
}
