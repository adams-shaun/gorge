package ir

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ZoneSet is a set of zones, bit 1<<Zone. It encodes as a list of zone names
// in Zone order.
type ZoneSet uint16

// Zone is a game zone.
type Zone uint8

const (
	ZoneNone Zone = iota
	ZoneBattlefield
	ZoneHand
	ZoneLibrary
	ZoneGraveyard
	ZoneExile
	ZoneStack
	ZoneCommand
	zoneCount
)

var zoneNames = [...]string{
	ZoneNone:        "none",
	ZoneBattlefield: "battlefield",
	ZoneHand:        "hand",
	ZoneLibrary:     "library",
	ZoneGraveyard:   "graveyard",
	ZoneExile:       "exile",
	ZoneStack:       "stack",
	ZoneCommand:     "command",
}

// String returns the lower-kebab name of k.
func (k Zone) String() string { return enumString(zoneNames[:], k, "Zone") }

// ParseZone is the inverse of String.
func ParseZone(s string) (Zone, bool) { return parseEnum[Zone](zoneNames[:], s) }

// MarshalJSON encodes k by name.
func (k Zone) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *Zone) UnmarshalJSON(b []byte) error { return unmarshalEnum(k, b, "Zone", ParseZone) }

// CmpOp is a numeric comparison operator.
type CmpOp uint8

const (
	CmpNone CmpOp = iota
	CmpEQ
	CmpNE
	CmpLT
	CmpLE
	CmpGT
	CmpGE
	cmpOpCount
)

var cmpOpNames = [...]string{
	CmpNone: "none",
	CmpEQ:   "eq",
	CmpNE:   "ne",
	CmpLT:   "lt",
	CmpLE:   "le",
	CmpGT:   "gt",
	CmpGE:   "ge",
}

// String returns the lower-kebab name of k.
func (k CmpOp) String() string { return enumString(cmpOpNames[:], k, "CmpOp") }

// ParseCmpOp is the inverse of String.
func ParseCmpOp(s string) (CmpOp, bool) { return parseEnum[CmpOp](cmpOpNames[:], s) }

// MarshalJSON encodes k by name.
func (k CmpOp) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *CmpOp) UnmarshalJSON(b []byte) error { return unmarshalEnum(k, b, "CmpOp", ParseCmpOp) }

// Compare is a comparison against a value.
type Compare struct {
	Op  CmpOp  `json:"op,omitempty"`
	V   *Value `json:"v,omitempty"`
	Src string `json:"src,omitempty"`
}

// Has reports whether z contains zone.
func (z ZoneSet) Has(zone Zone) bool { return z&(1<<zone) != 0 }

// Zones returns the member zones in Zone order.
func (z ZoneSet) Zones() []Zone {
	var out []Zone
	for i := Zone(1); i < zoneCount; i++ {
		if z.Has(i) {
			out = append(out, i)
		}
	}
	return out
}

// names returns the member zone names in Zone order. Bits that name no zone
// appear as "?<bit>" so distinct values never collide.
func (z ZoneSet) names() []string {
	var out []string
	for _, zn := range z.Zones() {
		out = append(out, zn.String())
	}
	if extra := z &^ zoneSetAll; extra != 0 {
		out = append(out, fmt.Sprintf("?%d", uint16(extra)))
	}
	return out
}

// zoneSetAll is every valid member bit (Zone 1 up to the sentinel).
const zoneSetAll = ZoneSet(1<<zoneCount-1) &^ 1

// String is the canonical form: member names in Zone order joined by commas.
func (z ZoneSet) String() string { return strings.Join(z.names(), ",") }

// MakeZoneSet builds a set from zones.
func MakeZoneSet(zs ...Zone) ZoneSet {
	var z ZoneSet
	for _, v := range zs {
		z |= 1 << v
	}
	return z
}

// MarshalJSON encodes z as a list of zone names in Zone order.
func (z ZoneSet) MarshalJSON() ([]byte, error) { return json.Marshal(z.names()) }

// UnmarshalJSON decodes a list of zone names, rejecting unknown names, "none"
// and duplicates.
func (z *ZoneSet) UnmarshalJSON(b []byte) error {
	var names []string
	if err := json.Unmarshal(b, &names); err != nil {
		return fmt.Errorf("ir: ZoneSet must be a JSON list of names: %w", err)
	}
	var out ZoneSet
	for _, n := range names {
		zn, ok := ParseZone(n)
		if !ok || zn == ZoneNone {
			return fmt.Errorf("ir: unknown Zone name %q", n)
		}
		if out.Has(zn) {
			return fmt.Errorf("ir: duplicate Zone name %q", n)
		}
		out |= 1 << zn
	}
	*z = out
	return nil
}
