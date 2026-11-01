package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// This is the census pin: noShuffleBody forces
// NoShuffle$ True onto EVERY pile body, so every pile-body API that carries an
// unread-report path must read the key. ChangeZone and ChangeZoneAll do; Effect
// did not, so Fight or Flight's chosen pile (ChosenPile$ DBEffect, a
// `DB$ Effect` body with no library movement) reported
// "Effect ignores unread parameter(s) NoShuffle$" once per resolution. The key
// is meaningless for an Effect body, so the fix is to read (acknowledge) it,
// not to move it.

// fightOrFlightSA parses a TwoPiles sub-ability whose chosen pile is an Effect
// body in the shape of Fight or Flight's; the card set is fed from
// Ctx.Remembered so the note pin needs no battlefield.
func fightOrFlightSA(t *testing.T) *cards.SA {
	t.Helper()
	src := "Name:Fight or Flight\nTypes:Enchantment\nOracle:x\n" +
		"SVar:TrigTwoPile:DB$ TwoPiles | Defined$ You | Chooser$ Opponent | DefinedCards$ Remembered | Separator$ You | ChosenPile$ DBEffect\n"
	c, d := cards.ParseBytes("fight_or_flight.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	sa := cards.ResolveSVar(c.Faces[0].SVars, "TrigTwoPile")
	if sa == nil {
		t.Fatalf("TrigTwoPile did not resolve")
	}
	return sa
}

// fightOrFlightSVars is the SVar table Fight or Flight's pile body resolves
// through.
func fightOrFlightSVars() map[string]string {
	return map[string]string{
		"DBEffect":     "DB$ Effect | RememberObjects$ Remembered | StaticAbilities$ STCantAttack",
		"STCantAttack": "Mode$ CantAttack | ValidCard$ Creature.!IsRemembered",
	}
}

// TestTwoPilesEffectPileBodyDoesNotReportNoShuffle pins the census: an Effect
// pile body (Fight or Flight's chosen pile) emits no unread-parameter Note for
// the NoShuffle$ noShuffleBody forces onto it.
func TestTwoPilesEffectPileBodyDoesNotReportNoShuffle(t *testing.T) {
	t.Parallel()
	h, ids := twopilesBoard(t)
	sh := &curatorTapeHost{fakeHost: h, splitIdx: []int{0}, pickIdx: 0}
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:2]), SVars: fightOrFlightSVars()}
	Resolve(sh, ctx, fightOrFlightSA(t))
	for _, ev := range sh.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unread parameter") {
			t.Fatalf("a TwoPiles Effect pile body emitted an unread-parameter Note: %q", ev.Text)
		}
	}
}
