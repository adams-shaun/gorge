package ir

import "encoding/json"

// ValueKind is the shape of a Value.
type ValueKind uint16

const (
	VNone ValueKind = iota
	VConst
	VX
	VAdd
	VSub
	VMul
	VHalfDown
	VHalfUp
	VCount
	VPower
	VToughness
	VManaValue
	VLife
	VStartingLife
	VLifeGainedThisTurn
	VDevotion
	VCountersAmong
	VTotalPower
	VDistinctManaValues
	VSpellsCastBeforeThisTurn
	VChosenNumber
	VEventAmount
	VReceiptAmount
	VExcessDamage
	VCountReceipt
	valueKindCount
)

var valueKindNames = [...]string{
	VNone:                     "none",
	VConst:                    "const",
	VX:                        "x",
	VAdd:                      "add",
	VSub:                      "sub",
	VMul:                      "mul",
	VHalfDown:                 "half-down",
	VHalfUp:                   "half-up",
	VCount:                    "count",
	VPower:                    "power",
	VToughness:                "toughness",
	VManaValue:                "mana-value",
	VLife:                     "life",
	VStartingLife:             "starting-life",
	VLifeGainedThisTurn:       "life-gained-this-turn",
	VDevotion:                 "devotion",
	VCountersAmong:            "counters-among",
	VTotalPower:               "total-power",
	VDistinctManaValues:       "distinct-mana-values",
	VSpellsCastBeforeThisTurn: "spells-cast-before-this-turn",
	VChosenNumber:             "chosen-number",
	VEventAmount:              "event-amount",
	VReceiptAmount:            "receipt-amount",
	VExcessDamage:             "excess-damage",
	VCountReceipt:             "count-receipt",
}

// String returns the lower-kebab name of k.
func (k ValueKind) String() string { return enumString(valueKindNames[:], k, "ValueKind") }

// ParseValueKind is the inverse of String.
func ParseValueKind(s string) (ValueKind, bool) { return parseEnum[ValueKind](valueKindNames[:], s) }

// MarshalJSON encodes k by name.
func (k ValueKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *ValueKind) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "ValueKind", ParseValueKind)
}

// Value is a numeric expression.
type Value struct {
	Kind   ValueKind `json:"kind,omitempty"`
	N      int32     `json:"n,omitempty"`
	Args   []*Value  `json:"args,omitempty"`
	Filter *Filter   `json:"filter,omitempty"`
	Ref    *Ref      `json:"ref,omitempty"`
	// Ref2 is a second object reference, e.g. the spell for a "cast before
	// <spell>" count (VSpellsCastBeforeThisTurn).
	Ref2    *Ref        `json:"ref2,omitempty"`
	Word    string      `json:"word,omitempty"`
	Colors  ColorSet    `json:"colors,omitempty"`
	Receipt ReceiptKind `json:"receipt,omitempty"`
	Src     string      `json:"src,omitempty"`
}
