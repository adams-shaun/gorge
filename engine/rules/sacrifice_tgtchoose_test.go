package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRushOfDreadTargetOpponentChoosesSacrifice pins the meaning of Forge's
// inert `Mode$ TgtChoose` on a Sacrifice body: Rush of Dread's targeted
// opponent -- not the caster -- chooses which half (rounded up) of their own
// creatures to sacrifice (CR 701.21a; XMage's RushOfDreadSacrificeEffect).
// The decision is therefore posed to seat 1 with Min == Max == 2 over the
// three creatures it controls, and the log replays it exactly.
func TestRushOfDreadTargetOpponentChoosesSacrifice(t *testing.T) {
	t.Parallel()
	bear := card(t, "Name:Test Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	charms := make([]*cards.Card, 0, 40)
	charms = append(charms, corpusAlternativeCard(t, "Rush of Dread"))
	decks := [][]*cards.Card{
		append(charms, mountainDeck(t, 40-len(charms))...),
		append([]*cards.Card{bear, bear, bear}, mountainDeck(t, 37)...),
	}
	cfg := seatZeroStart(Config{Seed: 184, Names: []string{"a", "b"}, Decks: decks})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	moveSeededToHand(t, e, 0, "Rush of Dread")
	var victims []state.ObjID
	for i := 0; i < 3; i++ {
		victims = append(victims, moveByName(t, e, 1, "Test Bear", state.ZBattlefield))
	}
	// Precondition: the target really controls three creatures, so half
	// rounded up is a genuine choice (2 of 3) and seat 1 is a legal target.
	for _, id := range victims {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
			t.Fatalf("precondition: victim %d = %+v, want a battlefield creature controlled by seat 1", id, o)
		}
	}
	// ManaCost 1BB plus the sacrifice Spree mode's ModeCost$ 1.
	addMana(t, e, 0, "BBCC")

	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("after cast = %+v, want the Spree mode ask", d)
	}
	if d.Player != 0 {
		t.Fatalf("mode ask posed to seat %d, want the caster (0)", d.Player)
	}
	modeIdx := -1
	for _, o := range d.Options {
		if strings.Contains(o.Label, "sacrifices half") {
			modeIdx = o.Index
		}
	}
	if modeIdx < 0 {
		t.Fatalf("sacrifice mode not offered: %+v", d.Options)
	}
	submitChoices(t, e, modeIdx)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after the mode = %+v, want the target-opponent ask", d)
	}
	if d.Player != 0 {
		t.Fatalf("target ask posed to seat %d, want the caster (0)", d.Player)
	}
	targetIdx := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			targetIdx = o.Index
		}
	}
	if targetIdx < 0 {
		t.Fatalf("seat 1 not a legal target: %+v", d.Options)
	}
	submitChoices(t, e, targetIdx)

	// The target choice completes the cast; both seats pass priority so the
	// spell resolves to the mid-resolution sacrifice ask.
	for i := 0; i < 4; i++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		passIdx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				passIdx = o.Index
			}
		}
		if passIdx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d.Options)
		}
		submitChoices(t, e, passIdx)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "sacrifice" {
		t.Fatalf("after the target = %+v, want the sacrifice choice", d)
	}
	// The chooser is the targeted opponent, not the caster (the TgtChoose
	// meaning): posed to seat 1, over exactly its three creatures, half
	// rounded up == 2.
	if d.Player != 1 {
		t.Fatalf("sacrifice choice posed to seat %d, want the targeted opponent (1); caster choosing would be the defect", d.Player)
	}
	if d.Min != 2 || d.Max != 2 {
		t.Fatalf("sacrifice bounds = %d..%d, want 2..2 (half of three, rounded up)", d.Min, d.Max)
	}
	if len(d.Options) != 3 {
		t.Fatalf("sacrifice options = %d, want all three of seat 1's creatures: %+v", len(d.Options), d.Options)
	}
	for _, o := range d.Options {
		if o.Kind != "sacrifice" || o.Player != 1 {
			t.Fatalf("sacrifice option %+v, want a seat-1 creature", o)
		}
	}
	submitChoices(t, e, 0, 1)

	sacrificed := 0
	for _, id := range victims {
		switch e.G.Obj(id).Zone {
		case state.ZGraveyard:
			sacrificed++
		case state.ZBattlefield:
		default:
			t.Fatalf("victim %d ended in %s, want graveyard or battlefield", id, e.G.Obj(id).Zone)
		}
	}
	if sacrificed != 2 {
		t.Fatalf("sacrificed %d creatures, want 2", sacrificed)
	}
	replayCheck(t, e, cfg)
}
