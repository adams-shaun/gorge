package ir

import "encoding/json"

// Subject is what kind of thing a Filter selects.
type Subject uint8

const (
	SubjNone Subject = iota
	SubjObject
	SubjPermanent
	SubjPlayer
	SubjSpell
	SubjCard
	subjectCount
)

var subjectNames = [...]string{
	SubjNone:      "none",
	SubjObject:    "object",
	SubjPermanent: "permanent",
	SubjPlayer:    "player",
	SubjSpell:     "spell",
	SubjCard:      "card",
}

// String returns the lower-kebab name of k.
func (k Subject) String() string { return enumString(subjectNames[:], k, "Subject") }

// ParseSubject is the inverse of String.
func ParseSubject(s string) (Subject, bool) { return parseEnum[Subject](subjectNames[:], s) }

// MarshalJSON encodes k by name.
func (k Subject) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *Subject) UnmarshalJSON(b []byte) error { return unmarshalEnum(k, b, "Subject", ParseSubject) }

// FilterOp is a Filter's combinator.
type FilterOp uint8

const (
	FNone FilterOp = iota
	FAll
	FAny
	FNot
	FPred
	filterOpCount
)

var filterOpNames = [...]string{
	FNone: "none",
	FAll:  "all",
	FAny:  "any",
	FNot:  "not",
	FPred: "pred",
}

// String returns the lower-kebab name of k.
func (k FilterOp) String() string { return enumString(filterOpNames[:], k, "FilterOp") }

// ParseFilterOp is the inverse of String.
func ParseFilterOp(s string) (FilterOp, bool) { return parseEnum[FilterOp](filterOpNames[:], s) }

// MarshalJSON encodes k by name.
func (k FilterOp) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *FilterOp) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "FilterOp", ParseFilterOp)
}

// Pred is a filter predicate.
type Pred uint16

const (
	PNone Pred = iota
	PAny
	PCardType
	PSubtype
	PSupertype
	PColor
	PColorless
	PMulticolored
	PPermanentCard
	PManaValue
	PPower
	PToughness
	PControlledBy
	POwnedBy
	PIs
	PToken
	PTapped
	PAttacking
	PBlocking
	PHasKeyword
	PCounters
	POpponentOf
	PPlayerIs
	PAttackedThisTurn
	PControls
	PLifeTotal
	PLostLifeThisTurn
	PCardsInGraveyard
	PCardsInHand
	PControlsMoreThan
	PName
	predCount
)

var predNames = [...]string{
	PNone:             "none",
	PAny:              "any",
	PCardType:         "card-type",
	PSubtype:          "subtype",
	PSupertype:        "supertype",
	PColor:            "color",
	PColorless:        "colorless",
	PMulticolored:     "multicolored",
	PPermanentCard:    "permanent-card",
	PManaValue:        "mana-value",
	PPower:            "power",
	PToughness:        "toughness",
	PControlledBy:     "controlled-by",
	POwnedBy:          "owned-by",
	PIs:               "is",
	PToken:            "token",
	PTapped:           "tapped",
	PAttacking:        "attacking",
	PBlocking:         "blocking",
	PHasKeyword:       "has-keyword",
	PCounters:         "counters",
	POpponentOf:       "opponent-of",
	PPlayerIs:         "player-is",
	PAttackedThisTurn: "attacked-this-turn",
	PControls:         "controls",
	PLifeTotal:        "life-total",
	PLostLifeThisTurn: "lost-life-this-turn",
	PCardsInGraveyard: "cards-in-graveyard",
	PCardsInHand:      "cards-in-hand",
	PControlsMoreThan: "controls-more-than",
	PName:             "name",
}

// String returns the lower-kebab name of k.
func (k Pred) String() string { return enumString(predNames[:], k, "Pred") }

// ParsePred is the inverse of String.
func ParsePred(s string) (Pred, bool) { return parseEnum[Pred](predNames[:], s) }

// MarshalJSON encodes k by name.
func (k Pred) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *Pred) UnmarshalJSON(b []byte) error { return unmarshalEnum(k, b, "Pred", ParsePred) }

// Filter selects objects or players.
//
//   - FAll, FAny and FNot combine Args.
//   - FPred tests Pred with its operands: Word for type, subtype, supertype,
//     keyword, counter kind and name; Colors; Ref; and Cmp/Value for numeric
//     predicates.
//   - PControls takes Args[0] as the object filter.
//   - PControlsMoreThan takes Ref (the other player) and Args[0].
type Filter struct {
	Subject Subject   `json:"subject,omitempty"`
	Zones   ZoneSet   `json:"zones,omitempty"`
	Op      FilterOp  `json:"op,omitempty"`
	Args    []*Filter `json:"args,omitempty"`
	Pred    Pred      `json:"pred,omitempty"`
	Word    string    `json:"word,omitempty"`
	Colors  ColorSet  `json:"colors,omitempty"`
	Ref     *Ref      `json:"ref,omitempty"`
	Cmp     CmpOp     `json:"cmp,omitempty"`
	Value   *Value    `json:"value,omitempty"`
	Src     string    `json:"src,omitempty"`
}
