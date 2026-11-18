package events

import "github.com/adams-shaun/gorge/engine/state"

// NamedAttachEntryCounter marks a face-up battlefield-entry MoveZone whose
// effect NAMES what the entering Aura enters attached to (ChangeZone's
// AttachedTo$/AttachedToPlayer$, DigUntil's revealed-Aura bearer, a copy
// token's AttachedTo$). The named bearers ride IDs in the effect's order: an
// object id, or a seat encoded with state.PlayerRef. An empty IDs list is a
// named set that resolved to nothing.
//
// CR 303.4f: an effect that specifies what the Aura enchants leaves its
// controller no choice; CR 303.4g: when nothing it names is something the
// Aura can legally enchant, the Aura stays in its zone. The rules engine's
// entry gate (engine/rules/aura_entry.go) reads the marker before the move folds and
// attaches the first legal named bearer as it does.
//
// MoveZone's Counter is otherwise unused on a face-up battlefield entry (the
// face-down/cloak markers and the exile markers are read only for those
// entries), and IDs is read only for an exile destination, so the fold
// ignores both and replay folds the marked move exactly like an unmarked one.
const NamedAttachEntryCounter = "attach-named"

// MarkNamedAttachEntry tags ev, a face-up battlefield entry, with its
// effect's named bearers.
func MarkNamedAttachEntry(ev *Event, named []state.ObjID) {
	ev.Counter = NamedAttachEntryCounter
	ev.IDs = append(ev.IDs[:0:0], named...)
}

// NamedAttachEntry returns the named bearers of a marked entry, and whether
// ev is one.
func NamedAttachEntry(ev *Event) ([]state.ObjID, bool) {
	if ev.Kind != MoveZone || ev.Counter != NamedAttachEntryCounter {
		return nil, false
	}
	return ev.IDs, true
}

// ChooseAttachEntryCounter marks a face-up battlefield-entry MoveZone whose
// effect names a CLASS of bearers (ChangeZone's AttachedTo$ card filter) and
// leaves the pick to the player putting the Aura onto the battlefield (CR
// 303.4f). IDs are the class's members the Aura's enchant ability admits, in
// the effect's order; the entry gate keeps those it can legally enchant (CR
// 702.16c's protection included) and poses the choice among them.
const ChooseAttachEntryCounter = "attach-choose"

// MarkChooseAttachEntry tags ev, a face-up battlefield entry, with the class
// of bearers its controller chooses among.
func MarkChooseAttachEntry(ev *Event, cands []state.ObjID) {
	ev.Counter = ChooseAttachEntryCounter
	ev.IDs = append(ev.IDs[:0:0], cands...)
}

// ChooseAttachEntry returns the class of a choose-marked entry, and whether
// ev is one.
func ChooseAttachEntry(ev *Event) ([]state.ObjID, bool) {
	if ev.Kind != MoveZone || ev.Counter != ChooseAttachEntryCounter {
		return nil, false
	}
	return ev.IDs, true
}
