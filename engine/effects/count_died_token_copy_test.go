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

// TestThisTurnEnteredCountsADeadTokenCopyEntry pins CR 111.1 / 700.14 /
// 603.10a for a battlefield DESTINATION: a token copy created this turn and
// killed the same turn still counts for the "created a token this turn"
// heads (Idol of Oblivion's Count$ThisTurnEntered_Battlefield_
// Card.tokenCreated+YouCtrl). The entry itself proves the copy was a
// permanent on the battlefield, so matchesZoneSpecCtx's CR 707.10h rejection
// (meant for a spell copy parked off the stack) must not drop it. A plain
// token is the control: it counted before the fix and must keep counting.
func TestThisTurnEnteredCountsADeadTokenCopyEntry(t *testing.T) {
	const body = "Count$ThisTurnEntered_Battlefield_Card.tokenCreated+YouCtrl"
	for _, tc := range []struct {
		name   string
		isCopy bool
		zone   state.Zone
	}{
		{"copy_died", true, state.ZGraveyard},
		{"copy_ceased", true, state.ZCeased},
		{"plain_died", false, state.ZGraveyard},
		{"plain_ceased", false, state.ZCeased},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(t, 2)
			g := h.g
			tok := g.AddObject(mkCard(t, "Name:Grizzly Bears\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
			tok.IsToken, tok.IsCopy = true, tc.isCopy
			tok.Zone = tc.zone
			g.Entered = append(g.Entered, state.ZoneEntry{Obj: tok.ID, To: state.ZBattlefield, From: state.ZLibrary})
			// Precondition: the token sits where the rule reads it (off the
			// battlefield) and the compared objects really differ on IsCopy.
			if !tok.IsToken || tok.IsCopy != tc.isCopy || tok.Zone != tc.zone {
				t.Fatalf("precondition: token/copy/zone = %v/%v/%v, want true/%v/%v",
					tok.IsToken, tok.IsCopy, tok.Zone, tc.isCopy, tc.zone)
			}
			if len(g.Entered) != 1 || g.Entered[0].To != state.ZBattlefield || g.Entered[0].From != state.ZLibrary {
				t.Fatalf("precondition: entered entry = %+v, want one battlefield entry from the library", g.Entered)
			}
			c := &Ctx{Controller: 0}
			if got, ok := EvalCountOK(h, c, body); !ok || got != 1 {
				t.Errorf("isCopy=%v zone=%v: count = (%d, %v), want (1, true)", tc.isCopy, tc.zone, got, ok)
			}
		})
	}
}
