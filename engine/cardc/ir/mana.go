package ir

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ColorSet is a set of colors, one bit each. It encodes as a string of
// letters in WUBRG order.
type ColorSet uint8

// The five colors.
const (
	ColorW ColorSet = 1 << iota
	ColorU
	ColorB
	ColorR
	ColorG
)

// ManaSym is one symbol of a mana cost. N is the generic amount, or 2 for
// ManaTwoHybrid.
type ManaSym struct {
	Kind   ManaKind `json:"kind,omitempty"`
	Colors ColorSet `json:"colors,omitempty"`
	N      int32    `json:"n,omitempty"`
}

// ManaKind is the shape of a mana symbol.
type ManaKind uint8

const (
	ManaNone ManaKind = iota
	ManaGeneric
	ManaColored
	ManaHybrid
	ManaPhyrexian
	ManaHybridPhyrexian
	ManaTwoHybrid
	ManaColorless
	ManaX
	ManaSnow
	manaKindCount
)

var manaKindNames = [...]string{
	ManaNone:            "none",
	ManaGeneric:         "generic",
	ManaColored:         "colored",
	ManaHybrid:          "hybrid",
	ManaPhyrexian:       "phyrexian",
	ManaHybridPhyrexian: "hybrid-phyrexian",
	ManaTwoHybrid:       "two-hybrid",
	ManaColorless:       "colorless",
	ManaX:               "x",
	ManaSnow:            "snow",
}

// String returns the lower-kebab name of k.
func (k ManaKind) String() string { return enumString(manaKindNames[:], k, "ManaKind") }

// ParseManaKind is the inverse of String.
func ParseManaKind(s string) (ManaKind, bool) { return parseEnum[ManaKind](manaKindNames[:], s) }

// MarshalJSON encodes k by name.
func (k ManaKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *ManaKind) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "ManaKind", ParseManaKind)
}

// colorLetters is the canonical letter order, matching the bit order.
const colorLetters = "WUBRG"

const allColors = ColorW | ColorU | ColorB | ColorR | ColorG

// String is the canonical form: the member letters in WUBRG order. Bits that
// are not colors print as "?<bits>" so distinct values never collide.
func (c ColorSet) String() string {
	var b []byte
	for i := 0; i < len(colorLetters); i++ {
		if c&(1<<i) != 0 {
			b = append(b, colorLetters[i])
		}
	}
	if extra := c &^ allColors; extra != 0 {
		b = append(b, fmt.Sprintf("?%d", uint8(extra))...)
	}
	return string(b)
}

// ParseColorSet decodes a WUBRG-order letter string. Unknown letters, repeats
// and out-of-order letters are rejected, so each set has one spelling.
func ParseColorSet(s string) (ColorSet, bool) {
	var c ColorSet
	last := -1
	for i := 0; i < len(s); i++ {
		j := strings.IndexByte(colorLetters, s[i])
		if j < 0 || j <= last {
			return 0, false
		}
		c |= 1 << j
		last = j
	}
	return c, true
}

// MarshalJSON encodes c as its letter string.
func (c ColorSet) MarshalJSON() ([]byte, error) { return json.Marshal(c.String()) }

// UnmarshalJSON decodes a letter string.
func (c *ColorSet) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("ir: ColorSet must be a JSON string: %w", err)
	}
	v, ok := ParseColorSet(s)
	if !ok {
		return fmt.Errorf("ir: bad ColorSet %q", s)
	}
	*c = v
	return nil
}
