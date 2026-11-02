package kgen

import (
	"strings"
)

// effectEtbCounters lowers a SpellCast trigger's "that creature enters
// with [N additional] counters" Effect. Two body families
// (and):
//
//   - DB$ Effect | RememberObjects$ TriggeredCard | SetChosenNumber$ <svar>
//     | ExileOnMoved$ Stack | ReplacementEffects$ <Event$ Moved |
//     ValidCard$ Card.IsRemembered | Destination$ Battlefield |
//     ReplacementResult$ Updated | ReplaceWith$ <PutCounter ETB>>
//     (Torgal, A Fine Hound; Wildgrowth Archaic): the chosen-number count.
//   - the same Effect with a constant CounterNum$ 1 and a self-exiling
//     body, in the two spellings the Static$ True carriers print
//
// RememberObjects$ TriggeredCard and no
//
//	ExileOnMoved$ (Mikey & Don; the FINALITY trio), or ExileOnMoved$
//	Stack with ValidCard$ Card.EffectSource (Undead Sprinter).
//
// gorge effects/effect.go registers a live Moved replacement carrying the
// spell (ExileOnMoved where given; the persistent shape ends by its body's
// self-exile SubAbility) and rules/replacement.go applyReplacement applies
// the move, ends the effect, runs the PutCounter ETB body on the entered
// object and only then matches the entry's triggers. Lowered as
// EffEtbCounters / EffFinalityCtr: the spell's stack entry gains the value
// as extra enters-with counters. Any other shape returns false so
// effectPrevent's refusals stand; a shape this reads but does not recognise
// refuses by name.
func (lc *lowerCtx) effectEtbCounters(ps *params) (bool, error) {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "SpellCast" || !lc.trigBody || lc.ab.Latent {
		return false, nil
	}
	if _, ok := ps.m["StaticAbilities"]; ok {
		return false, nil
	}
	if _, ok := ps.m["Duration"]; ok {
		return false, nil
	}
	eom := ps.m["ExileOnMoved"]
	ro := ps.m["RememberObjects"]
	setN := ps.m["SetChosenNumber"]
	switch {
	case eom == "Stack" && ro == "TriggeredCard" && setN != "":
		ps.consume("ExileOnMoved", "RememberObjects", "SetChosenNumber")
	case ro == "TriggeredCard" && eom == "" && setN == "":
		// the persistent shape: the body's SubAbility exiles the Effect
		// after its counter lands
		ps.consume("RememberObjects")
	case eom == "Stack" && ro == "" && setN == "":
		// the self-cast spelling (Undead Sprinter)
		ps.consume("ExileOnMoved")
	default:
		return false, nil
	}
	name := ps.str("ReplacementEffects")
	if strings.Contains(name, ",") {
		return false, nil
	}
	r := etbParams(lc.face.SVars[name])
	if r == nil || r["Event"] != "Moved" {
		return false, nil
	}
	rp := newParams(r)
	rp.consume("Event")
	if o, ok := rp.get("Origin"); ok {
		// gorge's applyReplacement gates the move's origin; only the
		// stack spelling appears on these bodies (an absent Origin$ is the
		// persistent shape's unrestricted move, as gorge registers it)
		if o != "Stack" {
			return false, refuse("Effect ETB-counters replacement Origin$ %q", o)
		}
	}
	if !rp.is("Destination", "Battlefield") || !rp.is("ReplacementResult", "Updated") {
		return false, nil
	}
	vc := rp.str("ValidCard")
	if vc != "Card.IsRemembered" && vc != "Card.EffectSource" {
		return false, nil
	}
	if vc == "Card.EffectSource" && !(eom == "Stack" && ro == "") {
		// gorge's EffectSource reads the effect's source card: only the
		// self-cast carrier spells it
		return false, refuse("Effect ETB-counters replacement ValidCard$ %q", vc)
	}
	body := etbParams(lc.face.SVars[rp.str("ReplaceWith")])
	if body == nil {
		return false, nil
	}
	if left := rp.unread(); len(left) > 0 {
		return false, refuse("Effect ETB-counters replacement unread %v", left)
	}
	if body["DB"] != "PutCounter" || body["Defined"] != "ReplacedCard" || body["ETB"] != "True" ||
		(body["CounterType"] != "P1P1" && body["CounterType"] != "FINALITY") {
		return false, refuse("Effect ETB-counters replacement body")
	}
	ct := body["CounterType"]
	switch {
	case setN != "":
		// the chosen-number shape: exactly the Torgal body, P1P1 only
		if ct != "P1P1" || body["CounterNum"] != "Count$ChosenNumber" || body["Sub"] != "" || body["SubAbility"] != "" || len(body) != 5 {
			return false, refuse("Effect ETB-counters replacement body")
		}
	default:
		// the widened shapes: a constant one counter and a self-exiling sub
		if body["CounterNum"] != "1" || body["Sub"] != "" || body["SubAbility"] == "" || len(body) != 6 {
			return false, refuse("Effect ETB-counters replacement body")
		}
		if err := lc.etbCtrSelfExile(body["SubAbility"]); err != nil {
			return false, err
		}
	}
	obj, err := lc.defined("TriggeredCard")
	if err != nil {
		return false, err
	}
	switch {
	case setN != "":
		x, err := lc.value(setN)
		if err != nil {
			return false, err
		}
		lc.emit(lInstr{Op: "EffEtbCounters", Obj: obj, A: x})
	case ct == "P1P1":
		lc.emit(lInstr{Op: "EffEtbCounters", Obj: obj, A: cst(1)})
	default:
		lc.emit(lInstr{Op: "EffFinalityCtr", Obj: obj})
	}
	return true, nil
}

// etbCtrSelfExile verifies a widened body's SubAbility is the exact
// self-exile the carriers spell (the Effect removes itself after the
// counter lands: DB$ ChangeZone | Defined$ Self | Origin$ Command |
// Destination$ Exile). Anything else refuses by name.
func (lc *lowerCtx) etbCtrSelfExile(name string) error {
	s := etbParams(lc.face.SVars[name])
	if s == nil || s["DB"] != "ChangeZone" || s["Defined"] != "Self" || s["Origin"] != "Command" || s["Destination"] != "Exile" || len(s) != 4 {
		return refuse("Effect ETB-counters self-exile sub %q", name)
	}
	return nil
}

// etbParams splits an SVar body "DB$ API | Key$ value | ..." (or a
// replacement line "Event$ Moved | ...") into its parameters; the DB$ head is
// returned under "DB". nil for an empty body.
func etbParams(body string) map[string]string {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	out := map[string]string{}
	for _, seg := range strings.Split(body, "|") {
		k, v, ok := strings.Cut(strings.TrimSpace(seg), "$")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}
