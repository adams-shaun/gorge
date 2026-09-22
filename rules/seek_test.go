package rules

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The api:Seek end-to-end leaf on a real compiled Seek SA: activation moves
// the sought cards library -> hand and emits exactly ONE events.Seek marker
// for the whole Num$ 3 action, which is what turns Vexyr, Ich-Tekik's Heir's
// "Whenever you seek one or more cards" from dormant groundwork into a live
// trigger -- exactly one Golem token, not three. A no-match seek emits
// neither the marker nor the trigger. Replay-verified throughout.
//
// The carrier is an authored fixture artifact (never a corpus .txt, per the
// licensing rule) with `A:AB$ Seek`, so the SA is compiled by the real cards
// parser and resolves through the engine's own AB activation path.

// seekRelicSrc is a tap-for-seek artifact whose seek filter is the parameter.
func seekRelicSrc(typeSpec string, num int) string {
	return "Name:Seek Relic\nTypes:Artifact\n" +
		"A:AB$ Seek | Cost$ T | Type$ " + typeSpec + " | Num$ " + strconv.Itoa(num) +
		" | SpellDescription$ Seek.\nOracle:x\n"
}

func seekMarkerCount(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Seek {
			n++
		}
	}
	return n
}

// seekEligibleInLibrary counts seat p's library cards matching a Type$ spec
// through a real card-face check, for the test's own precondition.
func seekEligibleInLibrary(t *testing.T, e *Engine, p state.PlayerID, typeName string) int {
	t.Helper()
	n := 0
	for _, id := range e.G.Zone(state.ZLibrary, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil {
			for _, ty := range o.Face().Types {
				if ty == typeName {
					n++
					break
				}
			}
		}
	}
	return n
}

func TestVexyrRealSeekEmitsOneMarkerForMultiCardSeek(t *testing.T) {
	vexyr := tokenReplCorpusCard(t, "Vexyr, Ich-Tekik's Heir")
	relic := cardByName(t, seekRelicSrc("Creature", 3))
	bear := cardByName(t, "Name:Grizzly Bears\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	seat0 := []*cards.Card{vexyr, relic}
	for i := 0; i < 15; i++ {
		seat0 = append(seat0, bear)
	}
	e, cfg := tokenReplGame(t, 101, seat0...)
	vexyrID := moveSeededCard(t, e, 0, vexyr, state.ZBattlefield)
	relicID := moveSeededCard(t, e, 0, relic, state.ZBattlefield)

	eligible := seekEligibleInLibrary(t, e, 0, "Creature")
	if eligible < 3 {
		t.Fatalf("precondition: only %d eligible creatures in seat 0's library, need 3", eligible)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))

	// Activate the Seek Relic's only ability and drain.
	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, relicID, 0).Index)
	passUntilStackEmpty(t, e, 40)

	handAfter := len(e.G.Zone(state.ZHand, 0))
	if handAfter-handBefore != 3 {
		t.Fatalf("seek moved %d cards, want exactly 3", handAfter-handBefore)
	}
	if got := seekMarkerCount(e); got != 1 {
		t.Fatalf("Num$ 3 seek emitted %d Seek markers, want exactly 1", got)
	}
	if got := triggerPushesFor(e, vexyrID); got != 1 {
		t.Fatalf("Vexyr had %d trigger pushes, want exactly 1 for one seek action", got)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Phyrexian Golem Token"); got != 1 {
		t.Fatalf("Vexyr created %d Golem tokens, want exactly 1", got)
	}
	replayCheck(t, e, cfg)
}

func TestVexyrRealSeekWithNoMatchEmitsNeitherMarkerNorTrigger(t *testing.T) {
	vexyr := tokenReplCorpusCard(t, "Vexyr, Ich-Tekik's Heir")
	relic := cardByName(t, seekRelicSrc("Enchantment", 3))
	bear := cardByName(t, "Name:Grizzly Bears\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	seat0 := []*cards.Card{vexyr, relic}
	for i := 0; i < 15; i++ {
		seat0 = append(seat0, bear)
	}
	e, cfg := tokenReplGame(t, 103, seat0...)
	vexyrID := moveSeededCard(t, e, 0, vexyr, state.ZBattlefield)
	relicID := moveSeededCard(t, e, 0, relic, state.ZBattlefield)

	// Precondition: the seek filter genuinely has no match, so the no-marker
	// assertion below cannot pass because the seek never ran.
	if n := seekEligibleInLibrary(t, e, 0, "Enchantment"); n != 0 {
		t.Fatalf("precondition: %d enchantments in library, the no-match seek is not a no-match", n)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))

	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, relicID, 0).Index)
	passUntilStackEmpty(t, e, 40)

	if got := len(e.G.Zone(state.ZHand, 0)) - handBefore; got != 0 {
		t.Fatalf("no-match seek moved %d cards, want 0", got)
	}
	if got := seekMarkerCount(e); got != 0 {
		t.Fatalf("no-match seek emitted %d Seek markers, want 0", got)
	}
	if got := triggerPushesFor(e, vexyrID); got != 0 {
		t.Fatalf("Vexyr had %d trigger pushes on a no-match seek, want 0", got)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Phyrexian Golem Token"); got != 0 {
		t.Fatalf("Vexyr created %d Golem tokens on a no-match seek, want 0", got)
	}
	replayCheck(t, e, cfg)
}
