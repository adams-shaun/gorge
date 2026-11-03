package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// echoLenderSrc lends an extra trigger to one creature of its controller's
// choosing for as long as it stays tapped.
const echoLenderSrc = `Name:Echo Lender
ManaCost:1 U
Types:Creature Advisor
PT:1/1
A:AB$ Effect | Cost$ T | ValidTgts$ Creature.Other+YouCtrl | StaticAbilities$ LendEcho | Duration$ UntilUntaps | RememberObjects$ Targeted | ExileOnMoved$ Battlefield | SpellDescription$ The chosen creature's triggered abilities trigger one more time while this stays tapped.
SVar:LendEcho:Mode$ Panharmonicon | ValidCard$ Card.IsRemembered | Description$ Lent echo.
Oracle:x
`

// echoWatcherSrc gains a life point for each land that enters under its
// controller.
const echoWatcherSrc = `Name:Echo Watcher
ManaCost:1
Types:Creature Scout
PT:1/1
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Land.YouCtrl | Execute$ WatcherGain | TriggerZones$ Battlefield
SVar:WatcherGain:DB$ GainLife | LifeAmount$ 1
Oracle:x
`

// TestEffectPanharmoniconDoublesRememberedTriggers pins an Effect-delivered
// trigger doubler (CR 603.2d): while the lender stays tapped, the creature it
// chose triggers one more time; once the lender untaps, or when it never
// chose, the trigger stays single. Another creature of the same kind is never
// doubled.
func TestEffectPanharmoniconDoublesRememberedTriggers(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)

	landLife := func(t *testing.T, choose bool, untapBefore bool) int32 {
		t.Helper()
		e := corpusEngine(t, reg, []*cards.Card{card(t, echoLenderSrc), card(t, echoWatcherSrc)}, nil)
		lender := onBoardCard(t, e, 0, card(t, echoLenderSrc))
		watcher := onBoardCard(t, e, 0, card(t, echoWatcherSrc))
		bystander := onBoardCard(t, e, 0, card(t, echoWatcherSrc))
		if choose {
			e.emit(events.Event{Kind: events.Tap, Obj: lender})
			face := e.G.Obj(lender).Face()
			effects.Resolve(e, &effects.Ctx{Source: lender, Controller: 0, SVars: face.SVars,
				Targets: []state.Target{{Obj: watcher}}}, face.Abilities[0])
			if untapBefore {
				e.emit(events.Event{Kind: events.Untap, Obj: lender})
			}
		}
		_ = bystander
		before := e.G.Players[0].Life
		playLandFromHand(t, e, "Mountain")
		answerQuiet(t, e, 60)
		return e.G.Players[0].Life - before
	}

	// Two watchers trigger once each; the chosen one triggers a second time.
	if got := landLife(t, false, false); got != 2 {
		t.Fatalf("baseline: %d life, want 2", got)
	}
	if got := landLife(t, true, false); got != 3 {
		t.Fatalf("lender tapped: %d life, want 3 (chosen watcher twice, the other once)", got)
	}
	if got := landLife(t, true, true); got != 2 {
		t.Fatalf("lender untapped again: %d life, want 2", got)
	}
}
