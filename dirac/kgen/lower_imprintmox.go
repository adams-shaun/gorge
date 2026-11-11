package kgen

import "strings"

// The imprint mox (Chrome Mox): "When this enters, you may exile a
// nonartifact, nonland card from your hand. {T}: Add one mana of any of the
// exiled card's colors." gorge writes the exiled card to the source's
// persistent Imprinted list (events.Imprint), clears the list when the
// source leaves the battlefield (apply_zone.go's battlefield departure, and
// the face's static Cleanup trigger), forgets a card that leaves exile (the
// static Pump ForgetImprinted$ trigger), and reads the list through the
// CR 607.2a exile gate (effects definedSpec Imprinted: only the linked
// cards still in exile). Because the face's text names ExiledWithSource,
// gorge also sets the exiled card's reverse ExiledWith scalar
// (exileProvenanceNeeded), live for that battlefield incarnation and cleared
// by every later move of the card. The kernel reads exactly that reverse
// record (genState.exw, exwRev at the source's current ver; kernel
// mana_imprint.go), which follows the same three events: the source leaving
// (its ver moves on), the card leaving exile (exwLeaveExile), a re-exile
// (exwExile). The two static bookkeeping triggers therefore lower to
// nothing, and the reflected ability lowers as AfManaReflect with no
// ManaAmong filter on a printed ability.
//
// The shape, exactly: one ability, AB$ ManaReflected Cost$ T, Valid$
// Defined.Imprinted, ColorOrType$ Color, ReflectProperty$ Is; three
// triggers and no static or replacement: T0 the self enters trigger whose
// body is one ChangeZone Imprint$ True from Hand to Exile (no target, no
// Defined$, no Remember); T1 the self leaves-the-battlefield Static$ trigger
// whose body is Cleanup ClearImprinted$ True alone; T2 the Static$ trigger
// on a card leaving exile with ValidCard$ Card.IsImprinted+ExiledWithSourceLKI
// whose body is Pump ForgetImprinted$ TriggeredCard alone.
func (lc *lowerCtx) imprintMoxShape() bool {
	f := lc.face
	if lc.granted > 0 || len(f.Abilities) != 1 || len(f.Triggers) != 3 || len(f.Statics) != 0 || len(f.Repls) != 0 {
		return false
	}
	is := func(m map[string]string, k, v string) bool { return strings.TrimSpace(m[k]) == v }
	only := func(m map[string]string, keys ...string) bool {
		for k := range m {
			ok := false
			for _, w := range keys {
				ok = ok || k == w
			}
			if !ok {
				return false
			}
		}
		return true
	}
	a := f.Abilities[0]
	if a.Kind != "AB" || a.API != "ManaReflected" || a.Sub != nil || !is(a.Params, "Cost", "T") || !is(a.Params, "Valid", "Defined.Imprinted") ||
		!is(a.Params, "ColorOrType", "Color") || !is(a.Params, "ReflectProperty", "Is") ||
		!only(a.Params, "Cost", "Valid", "ColorOrType", "ReflectProperty", "SpellDescription") {
		return false
	}
	t0, t1, t2 := &f.Triggers[0], &f.Triggers[1], &f.Triggers[2]
	if t0.Mode != "ChangesZone" || t1.Mode != "ChangesZone" || t2.Mode != "ChangesZone" {
		return false
	}
	if !is(t0.Params, "ValidCard", "Card.Self") || !is(t0.Params, "Origin", "Any") || !is(t0.Params, "Destination", "Battlefield") || t0.Params["Static"] != "" {
		return false
	}
	w := t0.Effect
	if w == nil || w.API != "ChangeZone" || w.Sub != nil || !is(w.Params, "Imprint", "True") || !is(w.Params, "Origin", "Hand") || !is(w.Params, "Destination", "Exile") {
		return false
	}
	for k, v := range w.Params {
		if k == "Defined" || k == "ValidTgts" || k == "DefinedPlayer" || k == "SubAbility" || strings.Contains(k, "Remember") || strings.Contains(v, "Remember") || strings.Contains(v, "Imprinted") {
			return false
		}
	}
	if !is(t1.Params, "ValidCard", "Card.Self") || !is(t1.Params, "Origin", "Battlefield") || !is(t1.Params, "Destination", "Any") || !is(t1.Params, "Static", "True") ||
		!only(t1.Params, "Mode", "ValidCard", "Origin", "Destination", "Execute", "Static") {
		return false
	}
	if c := t1.Effect; c == nil || c.API != "Cleanup" || c.Sub != nil || !is(c.Params, "ClearImprinted", "True") || !only(c.Params, "ClearImprinted") {
		return false
	}
	if !is(t2.Params, "ValidCard", "Card.IsImprinted+ExiledWithSourceLKI") || !is(t2.Params, "Origin", "Exile") || !is(t2.Params, "Destination", "Any") || !is(t2.Params, "Static", "True") ||
		!only(t2.Params, "Mode", "ValidCard", "Origin", "Destination", "Execute", "Static") {
		return false
	}
	if p := t2.Effect; p == nil || p.API != "Pump" || p.Sub != nil || !is(p.Params, "ForgetImprinted", "TriggeredCard") || !only(p.Params, "ForgetImprinted") {
		return false
	}
	return true
}
