package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A land that would enter tapped, played beside a "lands you control enter
// untapped" permanent, meets two Updated entry replacements, so its
// controller is asked their order (CR 616.1). The land play is still a land
// play (CR 305.1) and the land still enters (CR 603.2, 603.6a): the parked
// order ask must not settle the play before the move, or the Mode$
// LandPlayed trigger loses its in-flight land play and never fires.
const (
	entryOrderTappedLandSrc = "Name:Tapland\nTypes:Land\n" +
		"R:Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ TapOnEntry | Description$ x\n" +
		"SVar:TapOnEntry:DB$ Tap | ETB$ True | Defined$ Self\nOracle:x\n"
	entryOrderUntapperSrc = "Name:Untapper\nManaCost:2 G\nTypes:Creature Insect\nPT:2/4\n" +
		"R:Event$ Moved | Destination$ Battlefield | ValidCard$ Land.YouCtrl | ActiveZones$ Battlefield | ReplacementResult$ Updated | ReplaceWith$ UntapOnEntry | Description$ x\n" +
		"SVar:UntapOnEntry:DB$ Untap | Defined$ ReplacedCard | ETB$ True\nOracle:x\n"
	entryOrderLandPlaySrc = "Name:Playwatch\nManaCost:3\nTypes:Artifact\n" +
		"T:Mode$ LandPlayed | TriggerZones$ Battlefield | ValidCard$ Land | Execute$ PlayDraw | TriggerDescription$ x\n" +
		"SVar:PlayDraw:DB$ Draw | NumCards$ 1 | Defined$ TriggeredCardController\nOracle:x\n"
	entryOrderLandfallSrc = "Name:Fallwatch\nManaCost:2 R\nTypes:Enchantment\n" +
		"T:Mode$ ChangesZone | Destination$ Battlefield | Origin$ Any | TriggerZones$ Battlefield | ValidCard$ Land.YouCtrl | Execute$ FallDraw | TriggerDescription$ x\n" +
		"SVar:FallDraw:DB$ Draw | NumCards$ 1 | Defined$ You\nOracle:x\n"
	entryOrderGraveGrantSrc = "Name:Gravegrant\nManaCost:3\nTypes:Artifact\n" +
		"S:Mode$ Continuous | AffectedZone$ Graveyard | MayPlay$ True | Affected$ Land.YouOwn | Description$ x\nOracle:x\n"
)

func TestLandPlayEntryOrderKeepsBothTriggers(t *testing.T) {
	t.Parallel()
	for _, fromGrave := range []bool{false, true} {
		name := "hand"
		if fromGrave {
			name = "graveyard"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := mayPlayBase(t)
			onBoardGrant(t, e, 0, entryOrderUntapperSrc)
			playWatch := onBoardGrant(t, e, 0, entryOrderLandPlaySrc)
			fallWatch := onBoardGrant(t, e, 0, entryOrderLandfallSrc)
			var land state.ObjID
			if fromGrave {
				onBoardGrant(t, e, 0, entryOrderGraveGrantSrc)
				land = graveCard(e, card(t, entryOrderTappedLandSrc), 0, 0)
			} else {
				land = handCard(e, card(t, entryOrderTappedLandSrc), 0)
			}
			e.priorityRound()
			d := e.Pending()
			idx := -1
			if d != nil && d.Kind == decision.KPriority {
				for _, o := range d.Options {
					if o.Kind == "play_land" && o.Obj == land {
						idx = o.Index
					}
				}
			}
			if idx < 0 {
				t.Fatalf("land not offered: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{idx}}); err != nil {
				t.Fatalf("submit play_land: %v", err)
			}
			// Precondition: the two Updated entry replacements compete, so the
			// order ask is posed and the land has not moved yet.
			if d := e.Pending(); d == nil || d.Kind != decision.KReplacement {
				t.Fatalf("pending after the play = %+v, want the CR 616.1 order ask", d)
			}
			if z := e.G.Obj(land).Zone; z == state.ZBattlefield {
				t.Fatalf("land already on the battlefield before the order answer")
			}
			answerReplacementOrderAsk(t, e)
			if z := e.G.Obj(land).Zone; z != state.ZBattlefield {
				t.Fatalf("land zone after the answer = %v, want battlefield", z)
			}
			if got := e.G.Players[0].LandsPlayed; got != 1 {
				t.Fatalf("LandsPlayed = %d, want 1", got)
			}
			moved := -1
			for i, ev := range e.L.Events {
				if ev.Kind == events.MoveZone && ev.Obj == land && ev.To == state.ZBattlefield {
					moved = i
				}
			}
			if moved < 0 {
				t.Fatalf("no MoveZone of the land to the battlefield in the log")
			}
			if n := observedTriggerCount(e, playWatch); n != 1 {
				t.Fatalf("land-play triggers = %d, want 1 (CR 305.1)", n)
			}
			if n := observedTriggerCount(e, fallWatch); n != 1 {
				t.Fatalf("landfall triggers = %d, want 1 (CR 603.6a)", n)
			}
		})
	}
}
