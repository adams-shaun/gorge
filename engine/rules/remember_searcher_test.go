package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// RememberSearcher$ names the player who searched, not the cards found: an
// opponent who accepts counts even when the search finds nothing (CR 701.23b),
// and one who declines does not.
func TestCRRememberSearcherCountsPlayersWhoSearch(t *testing.T) {
	src := tapeZoneSorcery("Offered Search",
		"A:SP$ ChangeZone | Origin$ Library | Destination$ Battlefield | ChangeType$ TYPE | DefinedPlayer$ Opponent | RememberSearcher$ True | Optional$ True | ChangeNum$ 1 | Hidden$ True | SubAbility$ DBGain\n"+
			"SVar:DBGain:DB$ GainLife | LifeAmount$ Searchers | SubAbility$ Tidy\n"+
			"SVar:Tidy:DB$ Cleanup | ClearRemembered$ True\n"+
			"SVar:Searchers:Count$RememberedSize")
	for _, tc := range []struct {
		name, kind string
		accept     bool
		wantGain   int32
	}{
		{"accepted and found", "Land", true, 1},
		{"accepted and found nothing", "Enchantment", true, 1},
		{"declined", "Land", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before int32
			e, _ := runServedScenario(t, 2, 33301, 1, func(t *testing.T, e *Engine) {
				before = e.G.Players[0].Life
				driveCastPick(t, e, "Offered Search", "B", func(d *decision.Decision) []int {
					if d.ResumeKind == "search_confirm" && !tc.accept {
						return []int{1}
					}
					return tapePickYes(d)
				})
			}, strings.Replace(src, "ChangeType$ TYPE", "ChangeType$ "+tc.kind, 1))
			if got := e.G.Players[0].Life - before; got != tc.wantGain {
				t.Fatalf("gained %d life, want %d", got, tc.wantGain)
			}
		})
	}
}

// A later ability finds the searchers on the source: "each player who searched
// a library this way shuffles" shuffles the player who accepted and only them.
func TestRememberSearcherShufflesThoseWhoSearched(t *testing.T) {
	src := tapeZoneSorcery("Offered Search",
		"A:SP$ ChangeZone | Origin$ Library | Destination$ Battlefield | ChangeType$ Land | DefinedPlayer$ Player | RememberSearcher$ True | Optional$ True | NoShuffle$ True | ChangeNum$ 1 | Hidden$ True | SubAbility$ ShuffleThem\n"+
			"SVar:ShuffleThem:DB$ Shuffle | SubAbility$ Tidy | Defined$ Player.IsRemembered\n"+
			"SVar:Tidy:DB$ Cleanup | ClearRemembered$ True")
	var from int
	e, _ := runServedScenario(t, 2, 33302, 2, func(t *testing.T, e *Engine) {
		from = len(e.L.Events)
		driveCastPick(t, e, "Offered Search", "B", func(d *decision.Decision) []int {
			if d.ResumeKind == "search_confirm" && d.Player == 1 {
				return []int{1}
			}
			return tapePickYes(d)
		})
	}, src)
	shuffled := map[int]int{}
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.Shuffle {
			shuffled[int(ev.Player)]++
		}
	}
	if shuffled[0] != 1 || shuffled[1] != 0 {
		t.Fatalf("shuffles by seat: %v, want one for seat 0 and none for seat 1", shuffled)
	}
}

// SameNameAsEach$ is one search with one pick per named card, each among cards
// sharing that card's name.
func TestSameNameAsEachPicksOncePerNamedCard(t *testing.T) {
	src := tapeZoneSorcery("Twin Search",
		"A:SP$ ChangeZone | Origin$ Library | Destination$ Battlefield | ChangeType$ Card | SameNameAsEach$ Valid Land.YouCtrl")
	picks := 0
	e, _ := runServedScenario(t, 2, 33303, 2, func(t *testing.T, e *Engine) {
		moveByName(t, e, 0, "Mountain", state.ZBattlefield)
		moveByName(t, e, 0, "Mountain", state.ZBattlefield)
		driveCastPick(t, e, "Twin Search", "B", func(d *decision.Decision) []int {
			if d.ResumeKind != "search" {
				return tapePick(d)
			}
			picks++
			for _, o := range d.Options {
				if o.Label != "Mountain" {
					t.Fatalf("offered %q for a Mountain", o.Label)
				}
			}
			return []int{0}
		})
	}, src)
	if picks != 2 {
		t.Fatalf("%d picks, want one per Mountain", picks)
	}
	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != 4 {
		t.Fatalf("%d permanents, want the two Mountains and two found", got)
	}
}

// A ChooseCard root is not a modal root: a targeting link below it is chosen as
// the trigger goes on the stack (CR 603.3d), before the root's choice resolves.
func TestChooseCardRootAnnouncesItsTargetingLink(t *testing.T) {
	src := tapeZoneETB("Choosing Entrant",
		"SVar:TrigBody:DB$ ChooseCard | Choices$ Land.YouCtrl | Mandatory$ True | SubAbility$ TakeOne\n"+
			"SVar:TakeOne:DB$ ChangeZone | Destination$ Exile | Origin$ Battlefield | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 1")
	e, _ := kernelFixture(t, 2, 33304, src)
	moveByName(t, e, 0, "Mountain", state.ZBattlefield)
	var order []string
	driveCastPick(t, e, "Choosing Entrant", "B", func(d *decision.Decision) []int {
		order = append(order, string(d.Kind))
		return tapePick(d)
	})
	if len(order) < 1 || order[0] != string(decision.KTarget) {
		t.Fatalf("decisions in order %v, want the link's target first", order)
	}
}
