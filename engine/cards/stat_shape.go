package cards

// statPrimitive is the registry label for one S: line: "stat:"+Mode, or
// "stat:<Mode>.OtherShape" for a mode whose implemented family covers only
// one shape of the line (the repl:<Event>.OtherShape / api:Craft.OtherShape
// precedent), so registering the mode name cannot certify a shape the engine
// leaves inert.
func statPrimitive(s Static) string {
	if s.Mode == "CantAttach" && !CantAttachEquipmentShape(s) && !CantAttachAuraShape(s) {
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

// CantAttachAuraShape reports whether an S:Mode$ CantAttach line is one of
// the two implemented "another Aura can't enchant this bearer" shapes:
//
//   - ValidCard$ Aura.Other + Target$ <bearer relative to the source>: the
//     attached Aura itself is the restriction's source (Anti-Magic Aura,
//     Consecrate Land), so the bearer named by Target$ (Creature.EnchantedBy,
//     Land.EnchantedBy) is the one the source enchants and the second Aura is
//     any other. rules' cantAttachAuraBlocks enforces it on the Aura's
//     targeting (the spell's target census and the CR 608.2b recheck) and on
//     the CR 303.4f entry choice.
//   - ValidCardToTarget$ Aura.!Attached + Target$ <bearer>, optionally
//     IsPresent$-gated (Guardian Beast: only while untapped, and only
//     artifacts you control). The IsPresent$ gate is read by rules'
//     presentGate, the one static IsPresent$ home.
//
// Both are attach-side only: a restriction never un-attaches (Guardian
// Beast's printed "This effect doesn't remove Auras already attached" is the
// same rule stated per card), so nothing reads these shapes from the CR
// 704.5m detach SBA. Anything else fails closed and stays
// stat:CantAttach.OtherShape.
func CantAttachAuraShape(s Static) bool {
	tgt := s.ParamStr(PKTarget)
	if tgt == "" {
		return false
	}
	switch s.ParamStr(PKValidCard) {
	case "Aura.Other":
		for k := range s.Params {
			switch k {
			case "Mode", "ValidCard", "Target", "Description":
			default:
				return false
			}
		}
		return true
	}
	switch s.ParamStr(PKValidCardToTarget) {
	case "Aura.!Attached":
		for k := range s.Params {
			switch k {
			case "Mode", "ValidCardToTarget", "Target", "IsPresent", "Description":
			default:
				return false
			}
		}
		return true
	}
	return false
}

// CantAttachEquipmentShape reports whether an S:Mode$ CantAttach line is the
// implemented Equipment shape: ValidCard$ names the attaching Equipment
// (Card.Self on the Equipment itself -- O-Naginata, Konda's Banner, Gate
// Smasher -- or Card.Equipment on the bearer -- Goblin Brawler), Target$ the
// bearer it can't be attached to, and nothing else scopes it. rules'
// cantAttachForbids enforces it through the Equipment arms of the
// equip/attach refusal and the CR 704.5n SBA. The Aura shapes (Anti-Magic
// Aura's Aura.Other, Guardian Beast's ValidCardToTarget$/IsPresent$) are
// CantAttachAuraShape's, read at the Aura's own enforcement points.
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
