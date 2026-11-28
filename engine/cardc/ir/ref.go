package ir

import "encoding/json"

// RefKind says which object or player a Ref denotes.
type RefKind uint16

const (
	RefNone RefKind = iota
	RefSelf
	RefYou
	RefTarget
	RefEventObject
	RefEventPlayer
	RefControllerOf
	RefOwnerOf
	RefAttachedTo
	RefActivePlayer
	RefDefendingPlayer
	RefIterPlayer
	RefIterObject
	RefReceipt
	RefAll
	RefUnion
	RefPlayerOrControllerOf
	RefChoose
	RefTopOfLibrary
	RefMemory
	refKindCount
)

var refKindNames = [...]string{
	RefNone:                 "none",
	RefSelf:                 "self",
	RefYou:                  "you",
	RefTarget:               "target",
	RefEventObject:          "event-object",
	RefEventPlayer:          "event-player",
	RefControllerOf:         "controller-of",
	RefOwnerOf:              "owner-of",
	RefAttachedTo:           "attached-to",
	RefActivePlayer:         "active-player",
	RefDefendingPlayer:      "defending-player",
	RefIterPlayer:           "iter-player",
	RefIterObject:           "iter-object",
	RefReceipt:              "receipt",
	RefAll:                  "all",
	RefUnion:                "union",
	RefPlayerOrControllerOf: "player-or-controller-of",
	RefChoose:               "choose",
	RefTopOfLibrary:         "top-of-library",
	RefMemory:               "memory",
}

// String returns the lower-kebab name of k.
func (k RefKind) String() string { return enumString(refKindNames[:], k, "RefKind") }

// ParseRefKind is the inverse of String.
func ParseRefKind(s string) (RefKind, bool) { return parseEnum[RefKind](refKindNames[:], s) }

// MarshalJSON encodes k by name.
func (k RefKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *RefKind) UnmarshalJSON(b []byte) error { return unmarshalEnum(k, b, "RefKind", ParseRefKind) }

// ReceiptKind names the kind of objects an earlier step produced.
type ReceiptKind uint8

const (
	RcNone ReceiptKind = iota
	RcExiled
	RcChosen
	RcRevealed
	RcCreated
	RcSacrificed
	RcDiscarded
	RcMilled
	RcDestroyed
	RcReturned
	RcFound
	receiptKindCount
)

var receiptKindNames = [...]string{
	RcNone:       "none",
	RcExiled:     "exiled",
	RcChosen:     "chosen",
	RcRevealed:   "revealed",
	RcCreated:    "created",
	RcSacrificed: "sacrificed",
	RcDiscarded:  "discarded",
	RcMilled:     "milled",
	RcDestroyed:  "destroyed",
	RcReturned:   "returned",
	RcFound:      "found",
}

// String returns the lower-kebab name of k.
func (k ReceiptKind) String() string { return enumString(receiptKindNames[:], k, "ReceiptKind") }

// ParseReceiptKind is the inverse of String.
func ParseReceiptKind(s string) (ReceiptKind, bool) {
	return parseEnum[ReceiptKind](receiptKindNames[:], s)
}

// MarshalJSON encodes k by name.
func (k ReceiptKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *ReceiptKind) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "ReceiptKind", ParseReceiptKind)
}

// Memory names a per-source store an earlier step of the same card wrote. It
// is the unresolved form of a link between abilities: a reader that cannot yet
// say which step wrote the store names the store.
type Memory uint8

const (
	MemNone Memory = iota
	MemRemembered
	MemImprinted
	MemTriggerRemembered
	MemChosenCard
	MemChosenType
	MemChosenColor
	MemNamedCard
	MemExiledWithSource
	// MemDelayedRemembered is what a delayed trigger remembered when it was set up.
	MemDelayedRemembered
	// MemChosenPlayer is the player an earlier step chose.
	MemChosenPlayer
	memoryCount
)

var memoryNames = [...]string{
	MemNone:              "none",
	MemRemembered:        "remembered",
	MemImprinted:         "imprinted",
	MemTriggerRemembered: "trigger-remembered",
	MemChosenCard:        "chosen-card",
	MemChosenType:        "chosen-type",
	MemChosenColor:       "chosen-color",
	MemNamedCard:         "named-card",
	MemExiledWithSource:  "exiled-with-source",
	MemDelayedRemembered: "delayed-remembered",
	MemChosenPlayer:      "chosen-player",
}

// String returns the lower-kebab name of k.
func (k Memory) String() string { return enumString(memoryNames[:], k, "Memory") }

// ParseMemory is the inverse of String.
func ParseMemory(s string) (Memory, bool) { return parseEnum[Memory](memoryNames[:], s) }

// MarshalJSON encodes k by name.
func (k Memory) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *Memory) UnmarshalJSON(b []byte) error { return unmarshalEnum(k, b, "Memory", ParseMemory) }

// EventRole says which participant of the triggering or replaced event a
// RefEventObject or RefEventPlayer denotes. RoleNone on RefEventPlayer is the
// event's player; on RefEventObject it is the event's object.
type EventRole uint8

const (
	RoleNone EventRole = iota
	RoleCard
	RoleSource
	RoleAttacker
	RoleBlocker
	RoleTarget
	RoleActivator
	RoleReplaced
	eventRoleCount
)

var eventRoleNames = [...]string{
	RoleNone:      "none",
	RoleCard:      "card",
	RoleSource:    "source",
	RoleAttacker:  "attacker",
	RoleBlocker:   "blocker",
	RoleTarget:    "target",
	RoleActivator: "activator",
	RoleReplaced:  "replaced",
}

// String returns the lower-kebab name of k.
func (k EventRole) String() string { return enumString(eventRoleNames[:], k, "EventRole") }

// ParseEventRole is the inverse of String.
func ParseEventRole(s string) (EventRole, bool) { return parseEnum[EventRole](eventRoleNames[:], s) }

// MarshalJSON encodes k by name.
func (k EventRole) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *EventRole) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "EventRole", ParseEventRole)
}

// Ref denotes an object or a player.
//
//   - RefTarget uses Slot.
//   - RefControllerOf, RefOwnerOf, RefAttachedTo and RefPlayerOrControllerOf use Of.
//   - RefAll is every object or player matching Filter.
//   - RefChoose is N objects matching Filter, chosen by Of. With UpTo the
//     chooser may pick fewer than N, down to none (a hidden-zone search, CR
//     701.23b; "up to N"); without it exactly N, or all there are when fewer.
//   - RefTopOfLibrary is the top N cards of Of's library.
//   - For both, a nil Of means the acting player: the enclosing Step's Who,
//     else you.
//   - RefReceipt is the objects an earlier step of the same program produced,
//     of kind Receipt.
//   - RefIterPlayer and RefIterObject are the current element of the enclosing
//     for-each.
//   - RefMemory is the contents of the source's store named by Memory.
//   - RefEventObject and RefEventPlayer use Role.
//   - LKI asks for the object's last known information (an event object or a
//     memory read after the object left its zone).
type Ref struct {
	Kind RefKind `json:"kind,omitempty"`
	// Slot is the target declaration index.
	Slot   uint8   `json:"slot,omitempty"`
	Of     *Ref    `json:"of,omitempty"`
	Filter *Filter `json:"filter,omitempty"`
	// N is the count, for RefChoose and RefTopOfLibrary.
	N *Value `json:"n,omitempty"`
	// UpTo lets a RefChoose chooser pick fewer than N.
	UpTo    bool        `json:"upTo,omitempty"`
	Receipt ReceiptKind `json:"receipt,omitempty"`
	Union   []*Ref      `json:"union,omitempty"`
	// Memory is the store a RefMemory reads.
	Memory Memory `json:"memory,omitempty"`
	// Role is the event participant a RefEventObject or RefEventPlayer reads.
	Role EventRole `json:"role,omitempty"`
	// LKI reads the object's last known information.
	LKI bool   `json:"lki,omitempty"`
	Src string `json:"src,omitempty"`
}

// Target is one target declaration. The enclosing Program's Targets index is the
// RefTarget Slot.
type Target struct {
	Min int32 `json:"min,omitempty"`
	Max int32 `json:"max,omitempty"`
	// MaxV is set when the count is a value (X targets).
	MaxV     *Value  `json:"maxV,omitempty"`
	Filter   *Filter `json:"filter,omitempty"`
	Distinct bool    `json:"distinct,omitempty"`
	Src      string  `json:"src,omitempty"`
}
