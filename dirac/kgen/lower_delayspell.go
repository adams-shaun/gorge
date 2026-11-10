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
	if lc.granted > 0 {
		// gorge's registration names the grantee as its source, and the
		// grantee's face has none of the grantor's SVars: Execute$ cannot be
		// resolved at fire time (Way of the Cryomancer). The table-level
		// refusal is a defect to fix in gorge's granted registration, not a
		// lowering this build can carry.
		return refuse("delayed SpellCast in a granted ability")
	}
	if _, ok := ps.get("Static"); ok {
		// gorge resolves the body inline with no stack object
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
