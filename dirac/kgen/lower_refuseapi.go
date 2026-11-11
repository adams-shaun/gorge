package kgen

// apiRefusalWhy names, for an effect API kgen deliberately does not lower, why
// the kernel cannot model it. refuseAPI appends it to the plain "api X" reason
// (the refusal class stays api:X: refusalClasses reads the word after "api ").
var apiRefusalWhy = map[string]string{
	// gorge's PreventDamage (effects/prevent_damage.go) is a real CR 615
	// shield: one this-turn DamageDone replacement per chosen recipient that
	// prevents only the next N damage, depletes across events and lets a
	// partial prevention through. The kernel's this-turn prevention records
	// (genState.prevCol, Ext PreventColor / PreventDamageScope) prevent all or
	// none of an event and hold no amount, and its three damage paths (a
	// noncombat park, the frozen combat events, the CR 616.1 order ask) each
	// compute lifelink, infect and damage triggers from the whole amount. A
	// depleting shield needs a remaining-amount field in the fixed-size state
	// and a partial-prevention rewrite in all three paths, in lockstep with
	// gorge's order among prevention candidates: not bounded code. Healing
	// Salve.
	"PreventDamage": "no depleting prevent-the-next-N-damage shield in the kernel",
}

// refuseAPI is the refusal for an effect API the lowering has no arm for.
func refuseAPI(api string) error {
	if why, ok := apiRefusalWhy[api]; ok {
		return refuse("api %s: %s", api, why)
	}
	return refuse("api %s", api)
}
