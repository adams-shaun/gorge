package kgen

// controlSpell lowers api:ControlSpell Mode$ Gain (Commandeer; gorge
// effects/choose_control.go effControlSpell): "Gain control of target
// noncreature spell" takes the cast's spell target off its caster's hands —
// one ExtControlSpell over the stack entry, no duration, the permanent it
// becomes entering under Commandeer's controller (kernel
// act_controlspell.go). The node's own ValidTgts$ Card.nonCreature beside
// TargetType$ Spell declares the stack-spell slot (the same slot shape a
// counterspell's or Brazen Bluff's head declares; gorge reads TargetType$
// as the target's legality). The corpus's only Mode$ value is Gain;
// NewController$ (gorge controlPlayer binding a named player), any other
// Mode$, and a chain that already holds slots refuse.
func (lc *lowerCtx) controlSpell(ps *params) error {
	if m := ps.str("Mode"); m != "Gain" {
		return refuse("ControlSpell Mode$ %q: only Gain transfers a stack spell's controller (gorge effControlSpell's Gain arm)", m)
	}
	if v := ps.peek("NewController"); v != "" {
		return refuse("ControlSpell NewController$ %q: gorge binds the transfer to a named player (controlPlayer)", v)
	}
	if ps.str("TargetType") != "Spell" || ps.str("ValidTgts") == "" {
		return refuse("ControlSpell TargetType %q ValidTgts %q: the transfer reads the cast's one spell target (Commandeer's shape)", ps.str("TargetType"), ps.str("ValidTgts"))
	}
	if len(lc.chainT) != 0 {
		return refuse("ControlSpell beside a chain that already declares %d target slots: gorge's Defined transfers every target of the chain", len(lc.chainT))
	}
	ps.consume("TargetType")
	f, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return err
	}
	t := lc.addTarget(lTarget{F: f, Zone: zStack, Spell: true})
	lc.emit(lInstr{Op: "ControlSpell", Obj: lRef{Kind: rTarget, N: t}})
	return nil
}
