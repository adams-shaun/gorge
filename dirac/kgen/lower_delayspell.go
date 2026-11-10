package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// delayedSpellCast lowers a Mode$ SpellCast DelayedTrigger (gorge
// effDelayedTriggerSpellCast, CR 603.7): "when you next cast an instant or
// sorcery spell this turn, <Execute$>" -- a one-shot registration fired by
// its controller's next cast of a spell ValidCard$ matches, dropped at the
// turn change (ThisTurn$ True). The latent ability keeps the SpellCast
// trigger (Event, ValidCard$ filter, ValidActivatingPlayer$ You), so its
// body reads the cast spell as a printed SpellCast trigger's does (a
// CopySpellAbility's Defined$ TriggeredSpellAbility), and the kernel reads
// the filter at the cast (dirac/trig_delayedspell.go).
func (lc *lowerCtx) delayedSpellCast(ps *params) error {
	_, static := ps.get("Static")
	if static && !ps.is("Static", "True") {
		return refuse("delayed SpellCast Static$")
	}
	if !ps.is("ThisTurn", "True") {
		return refuse("delayed SpellCast without ThisTurn$")
	}
	if vp := ps.str("ValidActivatingPlayer"); vp != "You" {
		return refuse("delayed SpellCast ValidActivatingPlayer %q", vp)
	}
	vc, ok := ps.get("ValidCard")
	if !ok {
		return refuse("delayed SpellCast without ValidCard$")
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	for _, c := range f.Clauses {
		if c.CmpX == "" {
			continue
		}
		// The kernel reads the bound at the cast from the registration's
		// source (fireDelayedSpell's evalCtx), so only a plain source power
		// is carried; a target's or a chosen value's CmpX stays refused
		// (Loki Laufeyson's cmcLEX).
		if c.CmpV == nil || c.CmpV.Kind != vPowerOf || c.CmpV.Ref.Kind != rSelf ||
			c.CmpV.Mul != 0 || c.CmpV.Div != 0 || c.CmpV.Add != 0 || c.CmpV.Neg {
			return refuse("delayed SpellCast filter reads %q", c.CmpX)
		}
	}
	exec := cards.ResolveSVar(lc.face.SVars, ps.str("Execute"))
	if exec == nil {
		return refuse("delayed trigger Execute %q", ps.str("Execute"))
	}
	if static {
		// gorge resolves a Static$ body inline at the cast, with no stack
		// object; only the "that spell can't be countered" Effect is
		// carried, as the kernel's uncounter record (dirac/repl_uncounterable.go)
		if !lc.uncounterEffect(exec) {
			return refuse("delayed SpellCast Static$")
		}
		ps.consume("Mode", "Static")
		lat := lc.newLatent("DelayedSpellUncounter")
		lat.Trigger = &lTrigger{Event: "SpellCast", F: f, Whose: whoYou}
		lc.emit(lInstr{Op: "DelayedTrigger", C: lat.Index, CS: "NextSpellCastUncounter"})
		return nil
	}
	if v := lc.chainNamesSVar(exec); v != "" {
		// the fired body resolves away from the registering ability: an
		// amount it reads off the source (Storm King's Thunder's xPaid, Sea
		// Gate Stormcaller's kicker) is not carried by the kernel's record
		return refuse("delayed SpellCast body reads %q", v)
	}
	ps.consume("Mode")
	lat := lc.newLatent("DelayedSpellCast")
	lat.Trigger = &lTrigger{Event: "SpellCast", F: f, Whose: whoYou}
	if err := lc.withAbility(lat, func() error { return lc.chain(exec, nil) }); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "DelayedTrigger", C: lat.Index, CS: "NextSpellCast"})
	return nil
}

// uncounterEffect reports whether exec is exactly the Effect "the triggering
// spell can't be countered while it is on the stack": one Counter
// replacement stopping the event (Layer$ CantHappen, no ReplaceWith$) over
// the remembered card when a spell is countered, remembering the triggering
// card and forgetting it when it leaves the stack (gorge
// counterReplacementMatchesAll's effect-created arm).
func (lc *lowerCtx) uncounterEffect(exec *cards.SA) bool {
	if exec.API != "Effect" || exec.Sub != nil {
		return false
	}
	ep := newParams(exec.Params)
	if !ep.is("RememberObjects", "TriggeredCard") || !ep.is("ForgetOnMoved", "Stack") {
		return false
	}
	name := ep.str("ReplacementEffects")
	ep.consume("RememberObjects", "ForgetOnMoved", "ReplacementEffects")
	if len(ep.unread()) != 0 || name == "" || strings.Contains(name, ",") {
		return false
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "Counter" {
		return false
	}
	rp := newParams(r.Params)
	if !rp.is("ValidCard", "Card.IsRemembered") || !rp.is("ValidSA", "Spell") || !rp.is("Layer", "CantHappen") {
		return false
	}
	rp.consume("Event", "ValidCard", "ValidSA", "Layer", "Description")
	return len(rp.unread()) == 0
}

// chainNamesSVar returns the first card SVar an SA chain's lines name (a
// SubAbility$ link counts), or "".
func (lc *lowerCtx) chainNamesSVar(sa *cards.SA) string {
	for n := sa; n != nil; n = n.Sub {
		for _, tok := range strings.FieldsFunc(n.Line, func(r rune) bool {
			return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
		}) {
			if _, ok := lc.face.SVars[tok]; ok {
				return tok
			}
		}
	}
	return ""
}
