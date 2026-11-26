package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effectRememberSelf reads an until-end-of-turn Effect's RememberObjects$
// Self whose trigger body names the remembered object only as Defined$
// Remembered over a SpellCast trigger (Mountain Titan: "put a +1/+1 counter
// on CARDNAME"). gorge remembers the Effect's source
// (effectRemembered's Self arm appends c.Source) and the delayed trigger's
// Defined$ Remembered reads that one object back; the kernel's emblem
// trigger is queued with the emblem's source object (State.g.embO) as its
// own, so Remembered lowers as Self (rememberedAlias over lc.remSelf). Any
// other Remembered read, in the trigger line or anywhere down the body
// chain, refuses by name: the kernel's per-resolution remembered set would
// read empty there.
func (lc *lowerCtx) effectRememberSelf(ps *params, t cards.Trigger, eot bool) error {
	if !eot {
		return refuse("Effect triggers with RememberObjects$ \"Self\" beside a Duration$")
	}
	for _, k := range sortedKeys(t.Params) {
		if strings.Contains(t.Params[k], "Remembered") {
			return refuse("Effect triggers with RememberObjects$ \"Self\" and trigger %s$ %q", k, t.Params[k])
		}
	}
	for sa := t.Effect; sa != nil; sa = sa.Sub {
		for _, k := range sortedKeys(sa.Params) {
			if v := sa.Params[k]; strings.Contains(v, "Remembered") && (k != "Defined" || strings.TrimSpace(v) != "Remembered") {
				return refuse("Effect triggers with RememberObjects$ \"Self\" and body %s$ %q", k, v)
			}
		}
	}
	if t.Mode != "SpellCast" {
		// the one mode with lockstep cover (effremself_lockstep_test.go); a
		// DamageDone one (Dovin, Grand Arbiter) diverges on the loyalty count
		// when several creatures deal combat damage at once
		return refuse("Effect triggers with RememberObjects$ \"Self\" over Mode$ %s", t.Mode)
	}
	ps.consume("RememberObjects")
	lc.remSelf = true
	return nil
}
