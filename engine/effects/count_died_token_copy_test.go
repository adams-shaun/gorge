package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestThisTurnEnteredCountsADeadTokenCopy pins CR 700.4 / 603.10a: a token
// copy of a creature (Extravagant Replication's Healer's Hawk) that died this
// turn died, even though the copy has since ceased to exist, so a morbid
// count -- Slumbering Cerberus' Count$ThisTurnEntered_Graveyard_from_
// Battlefield_Creature -- includes it. matchesZoneSpecCtx's CR 707.10h copy
// rejection (meant for a resolved spell copy parked off the stack) used to
// drop the entry. dirac census pairs 4548 11018 23248 52131 90596.
// A spell copy parked in exile keeps being skipped
// (TestThisTurnEnteredExileSkipsSpellCopies).
func TestThisTurnEnteredCountsADeadTokenCopy(t *testing.T) {
	h := newHost(t, 2)
	g := h.g
	hawk := g.AddObject(mkCard(t, "Name:Healer's Hawk\nManaCost:W\nTypes:Creature Bird\nPT:1/1\nOracle:x\n"), 0)
	hawk.IsCopy, hawk.IsToken = true, true
	hawk.Zone = state.ZGraveyard
	g.Entered = append(g.Entered, state.ZoneEntry{Obj: hawk.ID, To: state.ZGraveyard, From: state.ZBattlefield, WasCreature: true})
	c := &Ctx{Controller: 0}
	for _, body := range []string{
		"Count$ThisTurnEntered_Graveyard_from_Battlefield_Creature",
		"Count$ThisTurnEntered_Graveyard_from_Battlefield_Creature.YouCtrl",
		"Count$ThisTurnEntered_Graveyard_Creature",
	} {
		if got, ok := EvalCountOK(h, c, body); !ok || got != 1 {
			t.Errorf("%s = (%d, %v), want (1, true)", body, got, ok)
		}
	}
}
