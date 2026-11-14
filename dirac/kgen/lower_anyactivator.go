package kgen

// anyActivator lowers an activated ability's Activator$ Player ("Any player
// may activate this ability", CR 602.2; gorge activatorAllows ->
// effects.MatchesPlayerSpecFrom with the spec "Player": every player) to
// kop.Ability.AnyActivator. The kernel offers such an ability to the
// permanent's controller through the ordinary walk and to every other living
// seat through dirac/cast_anyactivator.go, prices its mana against the
// activator's own sources and pool, and puts it on the stack under the
// activator (beginActivate's ctrl is s.prio), so every "you" its program reads
// is the activator (CR 109.5), exactly as gorge's resolution binds it.
//
// The modelled shape is a battlefield non-mana ability whose cost is plain
// mana, with no target and no activation window beyond "only as a sorcery"
// (Feral Hydra's PutCounter, Flailing Ogre's Pump, Sash and Waistcoat,
// Unmen's Manifest): the offer for another player then needs nothing of the
// controller's bookkeeping. Any other Activator$ spelling (You, Opponent, a
// property), a mana ability, a cost part beyond mana, a target, a limit or a
// controller-relative gate (PlayerTurn$, IsPresent$, CheckSVar$, ...: see
// anyActivatorWindow) refuses.
func (lc *lowerCtx) anyActivator(apiName string, ps *params, ab *lAbility, cost lCost) error {
	v, ok := ps.get("Activator")
	if !ok {
		return nil
	}
	if v != "Player" {
		return refuse("Activator$ %q", v)
	}
	if ab.Kind != akActivated || ab.Zone != 0 && ab.Zone != zBattlefield || apiName == "Mana" || apiName == "ManaReflected" {
		return refuse("Activator$ Player outside a battlefield non-mana AB$")
	}
	if !cost.pureMana() || cost.Mana.X || cost.Mana.XX || cost.Mana.XXX || cost.Mana.None || len(cost.Mana.Hybrid) != 0 ||
		len(cost.Mana.Twobrid) != 0 || len(cost.Mana.Phyrexian) != 0 || cost.Mana.XColor != 0 {
		return refuse("Activator$ Player with a cost other than plain mana")
	}
	if _, ok := ps.get("ValidTgts"); ok {
		return refuse("Activator$ Player with ValidTgts$")
	}
	if ab.Limit != 0 || ab.TurnOnce || ab.PowerUp || ab.Exhaust || ab.Reduce != nil || ab.Once != 0 || ab.Loyalty {
		return refuse("Activator$ Player with an activation limit or window")
	}
	ab.AnyAct = true
	return nil
}

// anyActivatorWindow refuses an Activator$ Player ability that came out of
// the lowering with an activate-only-if condition (PlayerTurn$, IsPresent$,
// CheckSVar$, Activation$, an activation limit) or a target anywhere in its
// chain (Wall of Vipers' sub-ability): the kernel evaluates the conditions and
// the target feasibility against the source's controller, while gorge's
// activationPhasesOK / abilityPresentHolds / target walk read the activator.
func anyActivatorWindow(ab *lAbility) error {
	if ab.AnyAct && (ab.ActCond.Kind != cNone || len(ab.Targets) != 0 || ab.Gate != nil) {
		return refuse("Activator$ Player with an activation condition or a target")
	}
	return nil
}
