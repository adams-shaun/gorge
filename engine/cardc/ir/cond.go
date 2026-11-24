package ir

import "encoding/json"

// CondKind is the shape of a Cond.
type CondKind uint16

const (
	CNone CondKind = iota
	CAll
	CAny
	CNot
	CCompare
	CRefMatches
	CExistsPlayer
	CCountCompare
	CCostPaid
	CIsTurnOf
	CNthTurn
	CReceiptNonEmpty
	CReceiptCount
	CDiedThisTurn
	CLeftBattlefieldThisTurn
	CKicked
	condKindCount
)

var condKindNames = [...]string{
	CNone:                    "none",
	CAll:                     "all",
	CAny:                     "any",
	CNot:                     "not",
	CCompare:                 "compare",
	CRefMatches:              "ref-matches",
	CExistsPlayer:            "exists-player",
	CCountCompare:            "count-compare",
	CCostPaid:                "cost-paid",
	CIsTurnOf:                "is-turn-of",
	CNthTurn:                 "nth-turn",
	CReceiptNonEmpty:         "receipt-non-empty",
	CReceiptCount:            "receipt-count",
	CDiedThisTurn:            "died-this-turn",
	CLeftBattlefieldThisTurn: "left-battlefield-this-turn",
	CKicked:                  "kicked",
}

// String returns the lower-kebab name of k.
func (k CondKind) String() string { return enumString(condKindNames[:], k, "CondKind") }

// ParseCondKind is the inverse of String.
func ParseCondKind(s string) (CondKind, bool) { return parseEnum[CondKind](condKindNames[:], s) }

// MarshalJSON encodes k by name.
func (k CondKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *CondKind) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "CondKind", ParseCondKind)
}

// Cond is a boolean condition.
type Cond struct {
	Kind    CondKind    `json:"kind,omitempty"`
	Args    []*Cond     `json:"args,omitempty"`
	L       *Value      `json:"l,omitempty"`
	R       *Value      `json:"r,omitempty"`
	Cmp     CmpOp       `json:"cmp,omitempty"`
	Filter  *Filter     `json:"filter,omitempty"`
	Ref     *Ref        `json:"ref,omitempty"`
	Word    string      `json:"word,omitempty"`
	Receipt ReceiptKind `json:"receipt,omitempty"`
	Src     string      `json:"src,omitempty"`
}
