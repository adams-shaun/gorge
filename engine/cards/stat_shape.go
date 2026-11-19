package cards

// statPrimitive is the registry label for one S: line: "stat:"+Mode, or
// "stat:<Mode>.OtherShape" for a mode whose implemented family covers only
// one shape of the line (the repl:<Event>.OtherShape / api:Craft.OtherShape
// precedent), so registering the mode name cannot certify a shape the engine
// leaves inert.
func statPrimitive(s Static) string {
	if s.Mode == "CantAttach" && !CantAttachEquipmentShape(s) {
		return "stat:CantAttach.OtherShape"
	}
	if s.Mode == "ManaRestriction" && !ManaRestrictionZeroSpendShape(s) {
		return "stat:ManaRestriction.OtherShape"
	}
	return "stat:" + s.Mode
}

// ManaRestrictionZeroSpendShape reports whether an S:Mode$ ManaRestriction
// line is the implemented zero-spend shape: "You can't spend mana to cast
// this spell" (Hogaak, Arisen Necropolis) -- ValidSource$ Card,Emblem with
// InvertValidSource$ True, scoped by nothing but EffectZone$ All, so the
// static applies from every zone its source sits in and restricts the casting
// of its own source. rules' manaSpendCastBan enforces it: every unit of that
// cast's cost must come from its Convoke and Delve credits, and any mana left
// makes the cast illegal. The mode's source-restricted shapes (Myr Superion's
// Creature.inZoneBattlefield, Imperiosaur's Land.Basic+inZoneBattlefield)
// need per-source pool provenance state.Player does not track, so they stay
// registered as stat:ManaRestriction.OtherShape.
func ManaRestrictionZeroSpendShape(s Static) bool {
	for k := range s.Params {
		switch k {
		case "Mode", "ValidSource", "InvertValidSource", "EffectZone", "Description":
		default:
			return false
		}
	}
	for _, r := range zeroSpendShape {
		if s.Params[r.key] != r.val {
			return false
		}
	}
	return true
}

// zeroSpendShape is the parameter values ManaRestrictionZeroSpendShape
// requires (the mode itself included, so callers need not re-check it).
var zeroSpendShape = [...]struct{ key, val string }{
	{"Mode", "ManaRestriction"},
	{"ValidSource", "Card,Emblem"},
	{"InvertValidSource", "True"},
	{"EffectZone", "All"},
}

// CantAttachEquipmentShape reports whether an S:Mode$ CantAttach line is the
// implemented Equipment shape: ValidCard$ names the attaching Equipment
// (Card.Self on the Equipment itself -- O-Naginata, Konda's Banner, Gate
// Smasher -- or Card.Equipment on the bearer -- Goblin Brawler), Target$ the
// bearer it can't be attached to, and nothing else scopes it. rules'
// cantAttachForbids enforces it through the Equipment arms of the
// equip/attach refusal and the CR 704.5n SBA. The Aura shapes (Anti-Magic Aura's
// Aura.Other, Guardian Beast's ValidCardToTarget$/IsPresent$) also restrict an
// Aura spell's targets, which nothing here gates, so they stay unregistered.
func CantAttachEquipmentShape(s Static) bool {
	for k := range s.Params {
		switch k {
		case "Mode", "ValidCard", "Target", "Description":
		default:
			return false
		}
	}
	v := s.ParamStr(PKValidCard)
	return (v == "Card.Self" || v == "Card.Equipment") && s.ParamStr(PKTarget) != ""
}
