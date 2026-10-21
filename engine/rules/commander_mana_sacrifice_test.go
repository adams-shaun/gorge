package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// altarFixtureSrc is Phyrexian Altar's shape, authored freely: a mana
// ability whose only cost is sacrificing a creature, producing one mana of
// any colour -- so its resolution poses a colour choice right after the
// sacrifice is paid.
const altarFixtureSrc = `Name:Fixture Altar
ManaCost:3
Types:Artifact
A:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ Any | SpellDescription$ Add one mana of any color.
Oracle:Sacrifice a creature: Add one mana of any color.
`

// TestManaAbilitySacrificingCommanderAsksTheCommandZoneAfterTheMana is the
// botbench livelock board (commander, foundations-reign-of-dragons vs
// rakdos-muscle-scam-exe, seed 105 game 1): Phyrexian Altar sacrificing
// Rakdos, the Muscle -- the controller's own commander and its only
// creature. Under CR 903.9a the sacrifice puts the commander into the
// graveyard at once (nothing is parked, so nothing can strand it on the
// battlefield), the mana ability's colour choice follows, and the owner's
// command-zone choice is the state-based action checked when the activating
// player next receives priority (CR 117.3c).
func TestManaAbilitySacrificingCommanderAsksTheCommandZoneAfterTheMana(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		choice int
		want   state.Zone
	}{
		{"accept", 0, state.ZCommand},
		{"decline", 1, state.ZGraveyard},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, cfg := colourIdentityGame(t, 11, FormatCommander, card(t, tinyCmdSrc), nil, card(t, altarFixtureSrc))
			toMain1(t, e)
			cmd := fieldCommander(t, e, 0, 0)
			altar := moveToBattlefieldByName(t, e, 0, "Fixture Altar")
			e.priorityRound()

			activateMana(t, e, altar)

			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose {
				t.Fatalf("pending after the altar's sacrifice = %v, want the altar's colour choice", pendingKind(d))
			}
			if got := e.G.Obj(cmd).Zone; got != state.ZGraveyard {
				t.Fatalf("sacrificed commander zone = %v, want graveyard (CR 903.9a: the sacrifice is not replaced)", got)
			}
			submit(t, e, 2) // Add B
			if got := e.G.Players[0].Pool[state.MB]; got != 1 {
				t.Fatalf("pool B = %d, want 1", got)
			}
			d = e.Pending()
			if d == nil || d.Kind != decision.KCommanderZone || d.Player != 0 {
				t.Fatalf("pending after the colour answer = %v, want seat 0's commander-zone choice", pendingKind(d))
			}
			submit(t, e, tc.choice)
			if got := e.G.Obj(cmd).Zone; got != tc.want {
				t.Fatalf("commander zone after answer = %v, want %v", got, tc.want)
			}
			if got := e.G.Players[0].Pool[state.MB]; got != 1 {
				t.Fatalf("pool B after the commander-zone answer = %d, want 1 (the mana stays)", got)
			}
			d = e.Pending()
			if d == nil || d.Kind != decision.KPriority {
				t.Fatalf("pending after the colour answer = %v, want priority", pendingKind(d))
			}
			for _, o := range d.Options {
				if o.Obj == altar {
					t.Fatalf("altar still offered with no creature left to sacrifice: %+v", o)
				}
			}
			asks := 0
			for _, ev := range e.L.Events {
				if ev.Kind == events.DecisionAsk && ev.Text == string(decision.KCommanderZone) {
					asks++
				}
			}
			if asks != 1 {
				t.Fatalf("commander-zone asks = %d, want 1", asks)
			}
			// A replay from Config + log alone reproduces the move and the mana
			// (fieldCommander's direct summoning-sickness clear is unlogged, so
			// the whole-state replayCheck diff does not apply here).
			re := commanderReplayFromLog(t, cfg, e.L.Events)
			if got := re.Obj(cmd).Zone; got != tc.want {
				t.Fatalf("log-only replay commander zone = %v, want %v", got, tc.want)
			}
			if got := re.Players[0].Pool[state.MB]; got != 1 {
				t.Fatalf("log-only replay pool B = %d, want 1", got)
			}
		})
	}
}

// pendingKind names a pending decision's kind for a failure message (the
// full struct is unreadably long).
func pendingKind(d *decision.Decision) string {
	if d == nil {
		return "<none>"
	}
	return string(d.Kind) + " " + d.Prompt
}
