package ir

import "encoding/json"

// Card is one printed card: its faces and the abilities lowered from them.
// Src fields hold the mtgish variant path that produced a node, never Oracle
// text.
type Card struct {
	Name   string  `json:"name,omitempty"`
	Layout Layout  `json:"layout,omitempty"`
	Faces  []*Face `json:"faces,omitempty"`
	Src    string  `json:"src,omitempty"`
}

// Layout is a card's physical layout.
type Layout uint8

const (
	LayoutNone Layout = iota
	LayoutNormal
	LayoutTransform
	LayoutModalDFC
	LayoutSplit
	LayoutAdventure
	LayoutFlip
	LayoutMeld
	LayoutPrepare
	LayoutRoom
	layoutCount
)

var layoutNames = [...]string{
	LayoutNone:      "none",
	LayoutNormal:    "normal",
	LayoutTransform: "transform",
	LayoutModalDFC:  "modal-dfc",
	LayoutSplit:     "split",
	LayoutAdventure: "adventure",
	LayoutFlip:      "flip",
	LayoutMeld:      "meld",
	LayoutPrepare:   "prepare",
	LayoutRoom:      "room",
}

// String returns the lower-kebab name of k.
func (k Layout) String() string { return enumString(layoutNames[:], k, "Layout") }

// ParseLayout is the inverse of String.
func ParseLayout(s string) (Layout, bool) { return parseEnum[Layout](layoutNames[:], s) }

// MarshalJSON encodes k by name.
func (k Layout) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *Layout) UnmarshalJSON(b []byte) error { return unmarshalEnum(k, b, "Layout", ParseLayout) }

// Face is one face of a card.
type Face struct {
	Name     string    `json:"name,omitempty"`
	ManaCost []ManaSym `json:"manaCost,omitempty"`
	// HasManaCost false means no mana cost, as distinct from {0}.
	HasManaCost    bool       `json:"hasManaCost,omitempty"`
	ColorIndicator ColorSet   `json:"colorIndicator,omitempty"`
	Supertypes     []string   `json:"supertypes,omitempty"`
	Types          []string   `json:"types,omitempty"`
	Subtypes       []string   `json:"subtypes,omitempty"`
	Power          *PT        `json:"power,omitempty"`
	Toughness      *PT        `json:"toughness,omitempty"`
	Loyalty        *int32     `json:"loyalty,omitempty"`
	Defense        *int32     `json:"defense,omitempty"`
	Abilities      []*Ability `json:"abilities,omitempty"`
	Refusals       []Refusal  `json:"refusals,omitempty"`
	Src            string     `json:"src,omitempty"`
}

// PT is a power or toughness. Star means a characteristic-defining *, with N
// added (*+1 is Star, N=1).
type PT struct {
	N    int32  `json:"n,omitempty"`
	Star bool   `json:"star,omitempty"`
	Src  string `json:"src,omitempty"`
}

// Refusal records a construct the lowering declined. Key is Enum.Variant;
// Path is the Src path where it was met.
type Refusal struct {
	Key  string `json:"key,omitempty"`
	Path string `json:"path,omitempty"`
}
