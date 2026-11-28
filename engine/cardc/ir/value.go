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
	VIf
	VCastThisTurn
	VEnteredThisTurn
	VDomain
	VPartySize
	VConverge
	VAttackersDeclared
	VTimesKicked
	VCardsDrawnThisTurn
	VMin
	VMax
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
	VIf:                       "if",
	VCastThisTurn:             "cast-this-turn",
	VEnteredThisTurn:          "entered-this-turn",
	VDomain:                   "domain",
	VPartySize:                "party-size",
	VConverge:                 "converge",
	VAttackersDeclared:        "attackers-declared",
	VTimesKicked:              "times-kicked",
	VCardsDrawnThisTurn:       "cards-drawn-this-turn",
	VMin:                      "min",
	VMax:                      "max",
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
//
//   - VIf is Args[0] when Test holds, else Args[1].
//   - VMin and VMax are the lesser and the greater of their two Args.
//   - VCastThisTurn counts the spells matching Filter cast this turn.
//   - VEnteredThisTurn counts the objects matching Filter that entered its
//     scope zone this turn, from Zones when Zones is set.
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
	// Test is VIf's condition.
	Test *Cond `json:"test,omitempty"`
	// Zones is VEnteredThisTurn's origin.
	Zones ZoneSet `json:"zones,omitempty"`
}
