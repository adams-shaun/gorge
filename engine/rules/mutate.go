package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Mutate (CR 702.140) is a cast mode on a creature card printed with
// K:Mutate:<cost>. Casting the spell for its mutate cost targets a non-Human
// creature its controller owns, and the mutating card is put over or under
// that creature as it resolves (CR 702.140b): the resulting permanent is the
// top card with all abilities of the cards beneath it (CR 702.140d), and a
// "whenever this creature mutates" trigger fires (CR 702.140f).
//
// This file owns the keyword's cast shape (mutateCost), the synthesized
// target declaration (mutateTargetSA) and the resolution that emits the
// events.Mutate fold. The pile itself is state.Object.MergedCards, folded by
// events.Apply (CR 702.140d) and read by the trigger/static scans.

// mutateCost resolves the keyword's mutate cost (CR 702.140a), Forge's
// K:Mutate:<cost>. It follows the replicateCost/bestowCost convention: a cost
// carrying a token ParseCost cannot model withholds the cast offer rather
// than charging a degraded generic. The mutating card's own printed mana cost
// is ignored -- the mutate cost is paid INSTEAD.
//
// The keyword parameter is occasionally followed by Forge's trailing fields;
// only the first colon-free field is the cost (no corpus mutate cost itself
// contains a colon -- they are all plain mana symbols and hybrids, measured
// over all 34 raw K:Mutate lines).
func mutateCost(f *cards.Face) (Cost, bool) {
	s, ok := f.KeywordParam("Mutate")
	if !ok {
		return Cost{}, false
	}
	cost, _, _ := strings.Cut(s, ":")
	c := ParseCost(strings.TrimSpace(cost))
	if len(c.Unknown) > 0 || c.X > 0 {
		return Cost{}, false
	}
	return c, true
}

// mutateTargetSA is the target declaration a mutate cast announces: CR
// 702.140a's "target non-Human creature you own". The creature face carries
// no SP of its own for the mutation, so this hand-built SA stands in for the
// face's spell ability at the target ask (engine/rules/cast.go) and at resolution
// (engine/rules/stack.go), exactly the bestowedAttachSA precedent -- never added to
// the face, which would make the PLAIN creature cast target-bearing.
//
// `Card.Self` is deliberately absent: the mutate target is the creature that
// becomes the pile's bottom (or top), not the spell's own card. YouOwn (not
// YouCtrl) is the oracle wording and the one the non-Human filter reads.
func mutateTargetSA() *cards.SA {
	return &cards.SA{Kind: "SP", API: "Mutate", Params: map[string]string{
		"ValidTgts": "Creature.nonHuman+YouOwn",
		"TgtPrompt": "Select target non-Human creature you own",
		"Keyword":   "Mutate",
	}}
}

// mutatePlaceAsk poses CR 702.140b's over-or-under choice once, during the
// cast (the "as this enters, choose ..." precedent: the choice is recorded at
// cast time so it can be read at resolution). It is a real two-option KChoose
// whose answer castAnswer folds onto pendingCast.mutateTop; the flag rides
// the pay-time CastInfo as FlagMutatedTop so a replay rebuilds the placement.
func (e *Engine) mutatePlaceAsk() bool {
	pc := e.cast
	if pc == nil || pc.ability >= 0 || pc.mode != "mutated" || pc.mutatePlaceDone {
		return false
	}
	pc.mutatePlaceDone = true
	o := e.G.Obj(pc.card)
	name := "this spell"
	if o != nil && o.Face() != nil {
		name = o.Face().Name
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Put " + name + " on top of or under the target creature",
		Source: pc.card}
	d.Options = []decision.Option{
		{Index: 0, Kind: "mutate_place", Label: "On top"},
		{Index: 1, Kind: "mutate_place", Label: "Under"},
	}
	// The under option carries Amount 0 and the top option Amount 1, so
	// castAnswer reads Option.Amount (never a positional Index) exactly as
	// the replicate/multikick arms do.
	d.Options[1].Amount = 0
	d.Options[0].Amount = 1
	e.choosing = chooseCast
	e.ask(d)
	return true
}

// mutateVictim is CR 702.140b's resolution-start check: the permanent a
// resolving mutating creature spell merges with, or 0 when its target is
// illegal. The mutating target is a sole-object target, revalidated with the
// same legalTargets helper every other spell's CR 608.2b recheck uses, so a
// target that left the battlefield or no longer matches
// Creature.nonHuman+YouOwn yields 0.
func (e *Engine) mutateVictim(o *state.Object) state.ObjID {
	sa := mutateTargetSA()
	for _, t := range e.legalTargets(o.Targets, sa, targetZones(sa), o.Controller, o.ID, o.ID) {
		if t.IsPlayer || t.Obj == 0 {
			continue
		}
		if to := e.G.Obj(t.Obj); to != nil && to.Zone == state.ZBattlefield {
			return t.Obj
		}
	}
	return 0
}

// resolveMutate lands a resolved mutate spell onto victim (mutateVictim's
// legal target): it emits the events.Mutate fold onto the surviving target
// permanent and returns WITHOUT the ordinary moveResolvedOffStack, because
// the mutating card does not become an independent permanent -- it merges
// into the target (CR 702.140c). The fold parks the spell's object, so this
// must run before any other move of that object. An illegal target never
// reaches here: resolveTop un-mutates the spell (CR 702.140b) and resolves
// it as a creature spell instead.
func (e *Engine) resolveMutate(o *state.Object, victim state.ObjID) {
	id := o.ID
	place := "under"
	if o.CastFlags&state.FlagMutatedTop != 0 {
		place = "top"
	}
	name := ""
	if f := o.Face(); f != nil {
		name = f.Name
	}
	e.emit(events.Event{Kind: events.Resolve, Obj: id, Text: name})
	e.emit(events.Event{Kind: events.Mutate, Obj: victim, IDs: []state.ObjID{id},
		Text: place, Amount: 1})
}

func init() {
	// Coverage: the keyword head engine/cards/primitive.go derives for every
	// K:Mutate line is now engine-supported (the mutate cast mode, the merged
	// pile and the Mutates trigger all live in rules), and the trigger mode is
	// registered so a Mode$ Mutates line no longer reports unsupported.
	effects.RegisterNonAPI("kw:Mutate", "trig:Mutates")
}
