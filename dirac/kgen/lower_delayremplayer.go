package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// delayedRemPlayer lowers a DelayedTrigger Mode$ Phase | Phase$ Upkeep |
// ValidPlayer$ Player | RememberObjects$ RememberedController: the
// countered spell's controller (a Counter's RememberTargets$ or
// RememberForCounter$ earlier in the chain) is the delayed program's
// remembered PLAYER (gorge effDelayedTrigger replaces the capture with
// knownDefinedTargets, which hands a player through; Defined$
// DelayTriggerRemembered then names that player). The kernel's delayed
// records hold objects, so the player rides the record as the fired
// program's trigger player (kop.DelayRemUpkeep); the body reads it as
// RefTriggerPlayer. Only the any-player upkeep shape lowers: a "You" upkeep
// has a different gate and refuses by name.
func (lc *lowerCtx) delayedRemPlayer(when string, exec *cards.SA) error {
	if when != "NextUpkeep" {
		return refuse("delayed upkeep trigger RememberObjects$ RememberedController beside ValidPlayer$ You")
	}
	obj, err := lc.rememberedController()
	if err != nil {
		return err
	}
	lat := lc.newLatent("DelayedTrigger")
	lat.DelayedPlayer = true
	if err := lc.withAbility(lat, func() error { return lc.chain(exec, nil) }); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "DelayedTrigger", Obj: obj, C: lat.Index, CS: "NextUpkeepOfRemPlayer"})
	return nil
}
