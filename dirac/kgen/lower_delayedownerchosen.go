package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// delayedOwnerOrChosen lowers a DelayedTrigger Mode$ Phase | Phase$ Upkeep |
// ValidPlayer$ Player | RememberObjects$ "TargetedOwner & ChosenPlayer"
// (Lodestone Bauble's "that player draws a card at the beginning of the next
// turn's upkeep"; the ChoosePlayer before it runs only when no card was
// targeted). gorge effDelayedTrigger replaces the capture with
// knownDefinedTargets of the two words: the owners of the resolving targets
// (deduplicated) and the resolution's chosen player. The two halves never
// both exist -- the ChoosePlayer's ConditionNotPresent$ Card on Targeted gates
// it on an empty target set -- so the registration remembers exactly one
// player, or none (a target set of no live object and no choice).
//
// The kernel's delayed player records hold one player and register nothing
// for a player ref that names nobody (delayRemUpkeep), so the lowering emits
// one NextUpkeepOfRemPlayer registration per half over one shared latent
// body: the owner of the target slot (kop.RefTargetOwner, the first LIVE target of the slot, as gorge's
// owners of the legal targets; the slot's targets share one controller, a
// graveyard card's owner) and the
// resolution's chosen player (kop.RefChosenPlayerRes). Whichever half is
// empty registers nothing. Defined$ DelayTriggerRemembered in the body reads
// the trigger player (lat.DelayedPlayer).
func (lc *lowerCtx) delayedOwnerOrChosen(when string, exec *cards.SA) error {
	if when != "NextUpkeep" {
		return refuse("delayed upkeep trigger RememberObjects$ \"TargetedOwner & ChosenPlayer\" beside ValidPlayer$ You")
	}
	if lc.chosenResAb == nil || lc.chosenResAb != lc.ab {
		return refuse("delayed trigger RememberObjects$ ChosenPlayer without a mid-resolution ChoosePlayer in this ability")
	}
	t := lc.lastTarget()
	if t < 0 || len(lc.chainT) != 1 {
		return refuse("delayed trigger RememberObjects$ TargetedOwner over %d target slots", len(lc.chainT))
	}
	if tg := lc.ab.Targets[t]; tg.F == nil || len(tg.F.Clauses) == 0 || tg.F.Players != 0 {
		return refuse("delayed trigger RememberObjects$ TargetedOwner over a slot that is not a card target")
	}
	lat := lc.newLatent("DelayedTrigger")
	lat.DelayedPlayer = true
	if err := lc.withAbility(lat, func() error { return lc.chain(exec, nil) }); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "DelayedTrigger", Obj: lRef{Kind: rTargetOwner, N: t}, C: lat.Index, CS: "NextUpkeepOfRemPlayer", Note: "the targets' owner"})
	lc.emit(lInstr{Op: "DelayedTrigger", Obj: lRef{Kind: rChosenPlayerRes}, C: lat.Index, CS: "NextUpkeepOfRemPlayer", Note: "the chosen player"})
	return nil
}
