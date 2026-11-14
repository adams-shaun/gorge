package kgen

// anyActivatorKeys are the only parameters an `Activator$ Player` ability may
// carry: the kernel offers it to every player (kop.Ability.AnyActivator,
// dirac/cast_anyactivator.go) through a deliberately narrow path, so everything
// that reads "the activator" beyond paying a mana cost is refused here.
var anyActivatorKeys = map[string]bool{
	"Cost": true, "Activator": true, "CounterType": true, "CounterNum": true,
	"SpellDescription": true, "StackDescription": true, "AILogic": true,
}

// anyActivator lowers an activated ability's Activator$ Player ("Any player
// may activate this ability", CR 602.2; gorge activatorAllows ->
// effects.MatchesPlayerSpecFrom with the spec "Player": every player). The
// shape is a battlefield `AB$ PutCounter | Cost$ <mana only> | CounterType$ K
// | CounterNum$ N` whose recipient is the source itself (Feral Hydra's "{3}:
// Put a +1/+1 counter on Feral Hydra. Any player may activate this
// ability."): nothing in it reads the activator's identity, so the kernel
// need only let another player pay the mana and put the ability on the
// stack. Any other Activator$ spelling (You, Opponent, a property), any
// other API, any cost part beyond mana, a target, a limit or a window gate
// refuses.
func (lc *lowerCtx) anyActivator(apiName string, ps *params, ab *lAbility, cost lCost) error {
	v, ok := ps.get("Activator")
	if !ok {
		return nil
	}
	if v != "Player" {
		return refuse("Activator$ %q", v)
	}
	if ab.Kind != akActivated || ab.Zone != 0 && ab.Zone != zBattlefield || apiName != "PutCounter" {
		return refuse("Activator$ Player outside a battlefield AB$ PutCounter")
	}
	if !cost.pureMana() || cost.Mana.X || cost.Mana.XX || cost.Mana.XXX || cost.Mana.None || len(cost.Mana.Hybrid) != 0 ||
		len(cost.Mana.Twobrid) != 0 || len(cost.Mana.Phyrexian) != 0 || cost.Mana.XColor != 0 {
		return refuse("Activator$ Player with a cost other than plain mana")
	}
	for _, k := range sortedParamKeys(ps.m) {
		if !anyActivatorKeys[k] {
			return refuse("Activator$ Player with %s$", k)
		}
	}
	if ab.Sorcery || ab.Limit != 0 || ab.TurnOnce || ab.PowerUp || ab.Exhaust || ab.Reduce != nil {
		return refuse("Activator$ Player with an activation limit or window")
	}
	ab.AnyAct = true
	return nil
}
