package kgen

import "strings"

// permOrSpellMove lowers ChangeZone Origin$ Battlefield,Stack with ValidTgts$
// <permanent filter>,Card.inZoneStack and TgtZone$ Stack,Battlefield or no
// TgtZone$ (Fatehold Charm, Jeskai Revelation; Swat Away,
// Sudden Setback): one target, a permanent
// matching the battlefield clauses or any spell other than this one (gorge
// effChangeZone's object path, filter_word.go inZoneStack, CR 115.5),
// moved from whichever of the two zones it is in (changeZoneMoveSkip).
//
// Bounce kop.MvPermOrSpell and OpToLibraryChoice's owner-asked top/bottom
// resume move a spell with the countered-spell move (so "can't be countered"
// does not stop it and a flashback-family cast is exiled instead: gorge
// keywordExileDeparture), a permanent as any bounce/library move.
func (lc *lowerCtx) permOrSpellMove(ps *params) error {
	ps.consume("Origin")
	vt := strings.TrimSpace(ps.str("ValidTgts"))
	var bf []string
	stack := 0
	for _, c := range strings.Split(vt, ",") {
		c = strings.TrimSpace(c)
		if c == "Card.inZoneStack" {
			stack++
			continue
		}
		bf = append(bf, c)
	}
	if stack != 1 || len(bf) == 0 {
		return refuse("Origin$ Battlefield,Stack ValidTgts$ %q", vt)
	}
	// An absent TgtZone$ targets both Origin$ zones (gorge
	// changeZoneTargetZones attributes the unmarked alternative to the
	// battlefield, Card.inZoneStack to the stack; Forge
	// AbilityFactory.adjustChangeZoneTarget): Swat Away, Sudden Setback.
	if tz, ok := ps.get("TgtZone"); ok && strings.TrimSpace(tz) != "Stack,Battlefield" {
		return refuse("Origin$ Battlefield,Stack TgtZone$ %q", tz)
	}
	ps.consume("TgtPrompt")
	dest, err := zoneOf(ps, "Destination", zNone)
	if err != nil {
		return err
	}
	var lib bool
	switch dest {
	case zHand:
		if _, ok := ps.get("AlternativeDecider"); ok {
			return refuse("Origin$ Battlefield,Stack to Hand with AlternativeDecider$")
		}
	case zLibrary:
		if _, ok := ps.get("LibraryPosition"); ok || ps.str("AlternativeDecider") != "TargetedOwner" ||
			ps.str("DestinationAlternative") != "Library" || ps.str("LibraryPositionAlternative") != "-1" {
			return refuse("Origin$ Battlefield,Stack to %s", zoneLabel[dest])
		}
		lib = true
	default:
		return refuse("Origin$ Battlefield,Stack to %s", zoneLabel[dest])
	}
	f, err := lc.filter(strings.Join(bf, ","))
	if err != nil {
		return err
	}
	if f.Players != 0 {
		return refuse("Origin$ Battlefield,Stack ValidTgts$ %q names players", vt)
	}
	tr := lRef{Kind: rTarget, N: lc.addTarget(lTarget{F: f, Zone: zBattlefield, PermOrSpell: true})}
	if lib {
		lc.emit(lInstr{Op: "ToLibrary", Obj: tr, C: 3, Note: "the owner chooses top or bottom (a spell too)"})
		return nil
	}
	lc.emit(lInstr{Op: "Bounce", Obj: tr, CS: "PermOrSpell", Note: "a permanent, or a spell off the stack"})
	return nil
}
