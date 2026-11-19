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
	return "stat:" + s.Mode
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
