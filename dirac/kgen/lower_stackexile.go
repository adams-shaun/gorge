package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// peek reads a parameter without marking it read: the hook runs on every
// ChangeZone, and only the Brazen Bluff shape may consume anything.
func (p *params) peek(k string) string { return strings.TrimSpace(p.m[k]) }

// pumpSpellTarget lowers a Pump that only carries a spell target: TargetType$
// Spell with TgtZone$ Stack and a ValidTgts$ spell filter and no pump
// parameter (Brazen Bluff's head: its chain acts on the target). gorge reads
// TargetType$ as the target's legality, so the slot is the same stack-spell
// slot a counterspell declares; the node itself does nothing. Any pump
// parameter beside it stays unread and refuses.
func (lc *lowerCtx) pumpSpellTarget(ps *params) error {
	if ps.str("TargetType") != "Spell" || ps.str("TgtZone") != "Stack" || ps.str("ValidTgts") == "" {
		return refuse("Pump TargetType %q TgtZone %q ValidTgts %q", ps.str("TargetType"), ps.str("TgtZone"), ps.str("ValidTgts"))
	}
	ps.consume("TargetType", "TgtZone")
	f, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return err
	}
	lc.addTarget(lTarget{F: f, Zone: zStack, Spell: true})
	return nil
}

// stackExileOptional lowers the chain link that moves the head's targeted
// spell to exile when its controller agrees: ChangeZone Defined$ Targeted
// Origin$ Stack Destination$ Exile Optional$ True, Chooser$ and
// DefinedPlayer$ TargetedController, RememberChanged$ True (gorge
// effChangeZone with changeZoneOptionalConfirm: the spell's controller is
// asked yes / no; a no drops the move only). It lowers to an OpMayAsk
// (MayGuardStackSpell, Obj the target's controller) over the countered-spell
// move of OpCounter with CtrDestExile, which still moves a spell that can't
// be countered, and CtrRemember adds the moved spell to the resolution's
// remembered set. Exactly one spell target slot is read.
func (lc *lowerCtx) stackExileOptional(ps *params) (bool, error) {
	if ps.peek("Origin") != "Stack" || ps.peek("Destination") != "Exile" || ps.peek("Defined") != "Targeted" {
		return false, nil
	}
	for _, kv := range [][2]string{{"Optional", "True"}, {"Chooser", "TargetedController"}, {"DefinedPlayer", "TargetedController"}, {"RememberChanged", "True"}} {
		if k := kv[0]; ps.peek(k) != kv[1] {
			return true, refuse("ChangeZone Origin$ Stack Destination$ Exile with %s$ %q", k, ps.peek(k))
		}
	}
	t := lc.lastTarget()
	if len(lc.chainT) != 1 || t < 0 {
		return true, refuse("ChangeZone Origin$ Stack Defined$ Targeted over %d target slots", len(lc.chainT))
	}
	tg := lc.ab.Targets[t]
	if !tg.Spell || tg.Zone != zStack || tg.StackAb || !isConstN(tg.Min, 1) || !isConstN(tg.Max, 1) {
		return true, refuse("ChangeZone Origin$ Stack Defined$ Targeted of a slot that is not exactly one spell")
	}
	who, err := lc.defined("TargetedController")
	if err != nil {
		return true, err
	}
	if who.Kind != rTargetController {
		return true, refuse("ChangeZone Chooser$ TargetedController as %s", who)
	}
	ps.consume("Origin", "Destination", "Defined", "Optional", "Chooser", "DefinedPlayer", "RememberChanged")
	lc.emit(lInstr{Op: "MayAsk", C: kop.MayGuardStackSpell, Obj: who, Skip: 1, Note: "the spell's controller may exile it"})
	lc.emit(lInstr{Op: "Counter", Obj: lRef{Kind: rTarget, N: t}, B: cst(kop.CtrRemember), C: kop.CtrDestExile | kop.CtrDestMove})
	return true, nil
}
