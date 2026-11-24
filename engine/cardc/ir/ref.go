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

// Ref denotes an object or a player.
//
//   - RefTarget uses Slot.
//   - RefControllerOf, RefOwnerOf, RefAttachedTo and RefPlayerOrControllerOf use Of.
//   - RefAll is every object or player matching Filter.
//   - RefChoose is N objects matching Filter, chosen by Of.
//   - RefTopOfLibrary is the top N cards of Of's library.
//   - For both, a nil Of means the acting player: the enclosing Step's Who,
//     else you.
//   - RefReceipt is the objects an earlier step of the same program produced,
//     of kind Receipt.
//   - RefIterPlayer and RefIterObject are the current element of the enclosing
//     for-each.
type Ref struct {
	Kind RefKind `json:"kind,omitempty"`
	// Slot is the target declaration index.
	Slot   uint8   `json:"slot,omitempty"`
	Of     *Ref    `json:"of,omitempty"`
	Filter *Filter `json:"filter,omitempty"`
	// N is the count, for RefChoose and RefTopOfLibrary.
	N       *Value      `json:"n,omitempty"`
	Receipt ReceiptKind `json:"receipt,omitempty"`
	Union   []*Ref      `json:"union,omitempty"`
	Src     string      `json:"src,omitempty"`
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
