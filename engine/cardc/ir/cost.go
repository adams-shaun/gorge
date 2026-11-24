package ir

import "encoding/json"

// CostKind is the kind of one cost component.
type CostKind uint8

const (
	CostNone CostKind = iota
	CostTapSelf
	CostUntapSelf
	CostSacrifice
	CostDiscard
	CostPayLife
	CostExile
	CostRemoveCounters
	CostReturnToHand
	CostTapUntapped
	CostReveal
	CostMill
	CostLoyalty
	costKindCount
)

var costKindNames = [...]string{
	CostNone:           "none",
	CostTapSelf:        "tap-self",
	CostUntapSelf:      "untap-self",
	CostSacrifice:      "sacrifice",
	CostDiscard:        "discard",
	CostPayLife:        "pay-life",
	CostExile:          "exile",
	CostRemoveCounters: "remove-counters",
	CostReturnToHand:   "return-to-hand",
	CostTapUntapped:    "tap-untapped",
	CostReveal:         "reveal",
	CostMill:           "mill",
	CostLoyalty:        "loyalty",
}

// String returns the lower-kebab name of k.
func (k CostKind) String() string { return enumString(costKindNames[:], k, "CostKind") }

// ParseCostKind is the inverse of String.
func ParseCostKind(s string) (CostKind, bool) { return parseEnum[CostKind](costKindNames[:], s) }

// MarshalJSON encodes k by name.
func (k CostKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *CostKind) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "CostKind", ParseCostKind)
}

// Cost is a mana cost plus other cost components.
type Cost struct {
	Mana    []ManaSym   `json:"mana,omitempty"`
	HasMana bool        `json:"hasMana,omitempty"`
	Parts   []*CostPart `json:"parts,omitempty"`
	Src     string      `json:"src,omitempty"`
}

// CostPart is one non-mana cost component.
type CostPart struct {
	Kind CostKind `json:"kind,omitempty"`
	Obj  *Ref     `json:"obj,omitempty"`
	N    *Value   `json:"n,omitempty"`
	Word string   `json:"word,omitempty"`
	Src  string   `json:"src,omitempty"`
}
