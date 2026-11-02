package cardset

import (
	"fmt"
	"strings"
)

// The interim layer guard (CR 613): the vocabulary
// and the check a generated matchup builder runs; dirac/kgen classifies each
// card and owns the conflict list (layerguard.go there), so a later layer
// change narrows the guard by editing that data.

// LayerClass is one order-sensitive effect class. Records (r*: an
// activated or triggered ability's override on an existing permanent) sit
// in application order, which is timestamp order, so two records never
// conflict; a static override (s*: an Aura's Witness Protection-style
// static) is always applied after every record, and a static subtype grant
// (gSub) after every override, whatever the timestamps say.
type LayerClass uint16

const (
	LcRecPT       LayerClass = 1 << iota // layer 7b record: sets base P/T (SetPT, Animate, an override with P/T)
	LcRecTypeSet                         // layer 4 record: sets card types or subtypes
	LcRecTypeAdd                         // layer 4 record: adds card types or subtypes (Animate, AddType)
	LcRecColor                           // layer 5 record: sets colours
	LcStatPT                             // layer 7b static override
	LcStatTypeSet                        // layer 4 static override setting types or subtypes
	LcStatTypeAdd                        // layer 4 static override adding types or subtypes
	LcStatColor                          // layer 5 static override
	LcGrantSub                           // layer 4 static subtype grant (StatContinuous AddSub)
	LcSubFilter                          // a static subtype grant whose filter reads subtypes (the kernel reads printed ones)
	LcFreshType                          // a type change on an object as it enters (ReturnEx then Override): always the oldest
)

var LayerClassNames = [...]string{"record base-P/T set", "record type set", "record type grant", "record colour set",
	"static base-P/T set", "static type set", "static type grant", "static colour set",
	"static subtype grant", "subtype-reading subtype grant", "entering type change"}

const (
	LcStatic     = LcStatPT | LcStatTypeSet | LcStatTypeAdd | LcStatColor
	LcAnyTypeChg = LcRecTypeSet | LcRecTypeAdd | LcStatTypeSet | LcStatTypeAdd | LcGrantSub | LcFreshType
)

func (c LayerClass) String() string {
	var s []string
	for i, n := range LayerClassNames {
		if c&(1<<i) != 0 {
			s = append(s, n)
		}
	}
	if len(s) == 0 {
		return "-"
	}
	return strings.Join(s, "+")
}

// LayerConflict is one refused pair of classes, from the plan's "Where it
// is wrong" table.
type LayerConflict struct {
	A, B LayerClass
	Why  string
}

// LayerClasses is one card's classes: Any holds every class it has, Other
// the classes of effects that can affect a permanent other than itself.
type LayerClasses struct {
	Any, Other LayerClass
	// Strips: a static override removes all abilities (Witness Protection,
	// Imprisoned in the Moon): the host's own self-only effects stop.
	Strips bool
	// Uncreature: a static override sets the card types without Creature.
	Uncreature bool
	// OnCreature: an Aura that can enchant only a creature.
	OnCreature bool
}

// LayerExempt reports whether card x's effect of class a and card y's of
// class b can never meet on one permanent in an order the kernel gets
// wrong: both affect only their own source; or x strips abilities with a
// static and y's effect is y's own (it stops once x enchants y; the
// residue -- y's ability already on the stack when x attaches -- is not
// guarded); or x makes its host a noncreature while y
// is a creature-only Aura (y falls off, CR 704.5m).
func LayerExempt(x, y *LayerClasses, a, b LayerClass) bool {
	if x.Other&a == 0 && y.Other&b == 0 {
		return true
	}
	if x.Strips && a&LcStatic != 0 && y.Other&b == 0 || y.Strips && b&LcStatic != 0 && x.Other&a == 0 {
		return true
	}
	if a&LcStatic != 0 && b&LcStatic != 0 && (x.Uncreature && y.OnCreature || y.Uncreature && x.OnCreature) {
		return true
	}
	return false
}

// LayerRefusal checks a set of cards (name, classes) and returns the first
// conflicting pair as an error naming both cards, or nil. Only distinct
// cards conflict: two copies of one card apply identical values.
func LayerRefusal(names []string, cls []LayerClasses, conflicts []LayerConflict) error {
	for _, c := range conflicts {
		for i := range names {
			a := cls[i].Any & c.A
			if a == 0 {
				continue
			}
			for j := range names {
				if i == j || names[i] == names[j] {
					continue
				}
				b := cls[j].Any & c.B
				if b == 0 || LayerExempt(&cls[i], &cls[j], a, b) {
					continue
				}
				return fmt.Errorf("layer guard: %q and %q: %s", names[i], names[j], c.Why)
			}
		}
	}
	return nil
}

// RoomNameRefusal refuses a matchup holding both a two-door Room (rooms:
// names of the pool's Rooms) and a card with a name-reading filter
// (readers; kop.F2SameNameAsSource, kop.PredNamed). A Room's name is its
// unlocked doors' (CR 709.5; gorge chars/room_name.go), but the kernel
// reads a name off the CardID, which a Room's door-state defs split, so a
// name read of a Room is not gorge's (refuse rather than guess).
func RoomNameRefusal(rooms, readers []string) error {
	if len(rooms) == 0 || len(readers) == 0 {
		return nil
	}
	return fmt.Errorf("Room name read: %s reads a name and %s is a Room (CR 709.5 door names are not modelled)", readers[0], rooms[0])
}

// AbReduceRefusal refuses a matchup holding both a ReduceCost static of
// activated abilities (reducers: kop.StatReduceAbility) and a mana ability
// with a mana cost (costed: a filter land). gorge's Type$ Ability reduction
// also prices a mana ability's mana cost, a walk the kernel does not
// compose (refuse rather than guess).
func AbReduceRefusal(reducers, costed []string) error {
	if len(reducers) == 0 || len(costed) == 0 {
		return nil
	}
	return fmt.Errorf("ReduceCost Type Ability beside a mana-costed mana ability: %s reduces abilities and %s has a mana ability with a mana cost", reducers[0], costed[0])
}
