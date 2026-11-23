package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.16b with CR 702.16c: a granted protection's exception field is the
// CR 702.16c removal carve-out ("this effect doesn't remove ..."), so it
// spares its named source on the state-based attachment check only — never
// the CR 702.16b targeting arm. A ward's own activation therefore still
// cannot target the creature it protects.
func TestProtectionExemptNoRemoveExceptionSparesAttachmentOnly(t *testing.T) {
	controller := func(state.ObjID) state.PlayerID { return 1 }
	hasSpec := func(spec string) bool { return spec == "Aura" }

	tattooWard := "Protection:Enchantment:enchantments:Card.CardUID_7"
	// Precondition: the exception names source 7, and the removal SBA is
	// spared by it.
	if !ProtectionExempt(tattooWard, 7, true, controller, hasSpec) {
		t.Fatal("precondition: the removal check must spare the named ward")
	}
	if ProtectionExempt(tattooWard, 8, true, controller, hasSpec) {
		t.Fatal("precondition: the exception names source 7 only")
	}
	// The fix: targeting (sba=false) is never spared.
	if ProtectionExempt(tattooWard, 7, false, controller, hasSpec) {
		t.Fatal("the targeting arm must not be spared by the removal exception")
	}

	// Benevolent Blessing's shape: the SBA-flagged clause list.
	benevolent := "Protection:Card.ChosenColor:chosenColor:" +
		"Aura.ControlledBy Player.PlayerUID_1,Equipment.ControlledBy Player.PlayerUID_1:SBA"
	if !ProtectionExempt(benevolent, 5, true, controller, hasSpec) {
		t.Fatal("precondition: the flagged clause list spares seat 1's Auras on the removal check")
	}
	if ProtectionExempt(benevolent, 5, false, controller, hasSpec) {
		t.Fatal("the flagged clause list must not spare the targeting arm")
	}

	// Spectra Ward's shape: a bare spec, already attachment-only.
	if !ProtectionExempt("Protection:Card.nonColorless:each color:Aura", 3, true, controller, hasSpec) {
		t.Fatal("precondition: the bare spec spares Auras on the removal check")
	}
	if ProtectionExempt("Protection:Card.nonColorless:each color:Aura", 3, false, controller, hasSpec) {
		t.Fatal("the bare spec must not spare the targeting arm")
	}
}
