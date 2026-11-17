package main

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/cardcov"
)

// The ability inventory, the use probe and the played-game walk live in
// testkit/cardcov so other drivers credit the same keys. What stays here is
// the unexported shape the tests read.

// abilitySlot is one cardcov.Slot as the tests read it.
type abilitySlot struct {
	key  string
	desc string
	mana bool
}

func abilityInventory(c *cards.Card) []abilitySlot {
	var out []abilitySlot
	for _, s := range cardcov.Inventory(c) {
		out = append(out, abilitySlot{key: s.Key, desc: s.Desc, mana: s.Mana})
	}
	return out
}

// abilityDescs maps key -> label for the report.
func abilityDescs(c *cards.Card) map[string]string { return cardcov.Descs(c) }

// realCardName walks an object's Source chain to the real card it came from.
func realCardName(e *rules.Engine, id state.ObjID) string { return cardcov.RealCardName(e, id) }
