package kgen

import "strings"

// Remembered persisting on the source (remembered-persist):
// gorge's Remembered also lives on the SOURCE object (effects eventRemember,
// read back through sourceRemembered), so Journey to Nowhere's leaves
// trigger returns the creature its enters trigger exiled, and Mesmeric
// Fiend's returns the card its enters trigger took. The kernel's remembered
// set is per resolution (resolve.go), so this one card shape is lowered with
// CardDef.RemPersist: the enters trigger's remembered objects are kept in
// the kernel's exile links (gen.links, keyed by the source's battlefield
// incarnation; zone_rememberpersist.go) when its resolution ends with the
// source still that incarnation, and the leaves trigger of that incarnation
// starts its resolution with them as its remembered set. A source that left
// before its enters trigger resolved has a new incarnation, which no leaves
// trigger reads (CR 610.3c's "O-ring trick": the card stays exiled).
//
// The shape, exactly: two triggers and nothing else that reads or writes
// Remembered. T0: Mode$ ChangesZone, Origin$ Any, Destination$ Battlefield,
// ValidCard$ Card.Self, whose body remembers (RememberTargets$ or
// RememberChanged$); T1: Mode$ ChangesZone, Origin$ Battlefield,
// Destination$ Any or Graveyard (the dies form, Hoarding Dragon),
// ValidCard$ Card.Self, whose body is ChangeZone Defined$
// Remembered Origin$ Exile to the battlefield or hand, optionally followed
// by Cleanup ClearRemembered$ True.
func (lc *lowerCtx) remPersistShape() bool {
	f := lc.face
	if len(f.Abilities) != 0 || len(f.Statics) != 0 || len(f.Repls) != 0 || len(f.Triggers) != 2 {
		return false
	}
	t0, t1 := &f.Triggers[0], &f.Triggers[1]
	if t0.Mode != "ChangesZone" || t1.Mode != "ChangesZone" {
		return false
	}
	is := func(m map[string]string, k, v string) bool { return strings.TrimSpace(m[k]) == v }
	if !is(t0.Params, "Origin", "Any") || !is(t0.Params, "Destination", "Battlefield") || !is(t0.Params, "ValidCard", "Card.Self") ||
		!is(t1.Params, "Origin", "Battlefield") || !(is(t1.Params, "Destination", "Any") || is(t1.Params, "Destination", "Graveyard")) || !is(t1.Params, "ValidCard", "Card.Self") {
		return false
	}
	w := t0.Effect
	if w == nil || w.Sub != nil || !(is(w.Params, "RememberTargets", "True") || is(w.Params, "RememberChanged", "True")) {
		return false
	}
	r := t1.Effect
	if r == nil || r.API != "ChangeZone" || !is(r.Params, "Defined", "Remembered") || !is(r.Params, "Origin", "Exile") ||
		!(is(r.Params, "Destination", "Battlefield") || is(r.Params, "Destination", "Hand")) {
		return false
	}
	if c := r.Sub; c != nil && (c.API != "Cleanup" || c.Sub != nil || !is(c.Params, "ClearRemembered", "True") || len(c.Params) != 1) {
		return false
	}
	for k, v := range w.Params {
		if strings.Contains(v, "Remembered") || k == "Defined" && strings.Contains(v, "Remember") {
			return false
		}
	}
	return true
}
