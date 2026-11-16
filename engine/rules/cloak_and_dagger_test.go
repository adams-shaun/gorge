package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cloakDaggerRun casts Cloak and Dagger, Entwined for p0 against p1 holding
// the given hand and a Grizzly Bears, answers the ETB's two targets (the
// opponent, then Bears when withBears), and returns the engine stopped at
// the "you may exile" choice -- the search-kind ask the chain's last link
// poses -- together with the Bears and hand-card ids.
func cloakDaggerRun(t *testing.T, hand []string, withBears bool) (*Engine, *oracleRun, *decision.Decision) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"Cloak and Dagger, Entwined"}},
			"p1": {Battlefield: []string{"Grizzly Bears"}, Hand: hand},
		},
		Steps: []oracleStep{{Op: "cast", Seat: 0, Card: "p0:Cloak and Dagger, Entwined", Mana: "CWB"}},
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) > 0 || run.e == nil {
		t.Fatalf("%v\ntranscript:\n%v", fails, transcript)
	}
	e := run.e
	bears, err := run.resolve("p1:Grizzly Bears")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("game ended before the exile choice")
		}
		switch {
		case d.Kind == decision.KPriority:
			submitChoicePass(t, e)
		case d.Kind == decision.KTarget && d.ResumeKind == "trig_sub":
			if !withBears {
				submitChoices(t, e)
				continue
			}
			if idx := indexOfObjOption(d, bears); idx >= 0 {
				submitChoices(t, e, idx)
			} else {
				t.Fatalf("the creature link does not offer Grizzly Bears: %+v", d.Options)
			}
		case d.Kind == decision.KTarget:
			cloakAnswerSeat(t, d, e, 1)
		case d.Kind == decision.KChoose && d.ResumeKind == "search":
			return e, run, d
		default:
			t.Fatalf("unexpected ask %+v", d)
		}
	}
	t.Fatal("never reached the exile choice")
	return nil, nil, nil
}

// cloakAnswerSeat submits the target-ask option naming the player.
func cloakAnswerSeat(t *testing.T, d *decision.Decision, e *Engine, p state.PlayerID) {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == p {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("seat %d not offered: %+v", p, d.Options)
}

func cloakZoneOf(e *Engine, id state.ObjID) state.Zone { return e.G.Obj(id).Zone }

// TestCloakAndDaggerOffersTheTargetedCreature: the exile is "a nonland card
// from their hand OR the chosen creature". With the creature link answered,
// the choice offers the chosen Grizzly Bears beside the hand's nonland card
// (never the land), and each answer exiles exactly the one picked.
func TestCloakAndDaggerOffersTheTargetedCreature(t *testing.T) {
	t.Parallel()
	for _, pick := range []string{"Grizzly Bears", "Lightning Bolt", ""} {
		t.Run("pick="+pick, func(t *testing.T) {
			e, run, d := cloakDaggerRun(t, []string{"Lightning Bolt", "Forest"}, true)
			bears, _ := run.resolve("p1:Grizzly Bears")
			bolt, _ := run.resolve("p1:Lightning Bolt")
			forest, _ := run.resolve("p1:Forest")
			if indexOfObjOption(d, forest) >= 0 {
				t.Fatalf("a land card is offered: %+v", d.Options)
			}
			if indexOfObjOption(d, bolt) < 0 || indexOfObjOption(d, bears) < 0 || len(d.Options) != 2 {
				t.Fatalf("exile choice = %+v, want exactly the Bolt and the chosen Bears", d.Options)
			}
			want := map[string]state.ObjID{"Grizzly Bears": bears, "Lightning Bolt": bolt}
			if pick == "" {
				submitChoices(t, e)
			} else {
				submitChoices(t, e, indexOfObjOption(d, want[pick]))
			}
			for name, id := range want {
				z := cloakZoneOf(e, id)
				if name == pick && z != state.ZExile {
					t.Fatalf("%s zone %s after being picked, want exile", name, z)
				}
				if name != pick && z == state.ZExile {
					t.Fatalf("%s exiled though %q was picked", name, pick)
				}
			}
		})
	}
}

// TestCloakAndDaggerEmptyHandExilesTheCreature is the XMage-replayed shape:
// p1's hand is empty, so the chosen creature is the only candidate and
// picking it leaves one card in p1's exile.
func TestCloakAndDaggerEmptyHandExilesTheCreature(t *testing.T) {
	t.Parallel()
	e, run, d := cloakDaggerRun(t, nil, true)
	bears, _ := run.resolve("p1:Grizzly Bears")
	idx := indexOfObjOption(d, bears)
	if idx < 0 || len(d.Options) != 1 {
		t.Fatalf("exile choice = %+v, want only the chosen Grizzly Bears", d.Options)
	}
	submitChoices(t, e, idx)
	if z := cloakZoneOf(e, bears); z != state.ZExile {
		t.Fatalf("Grizzly Bears zone %s, want exile", z)
	}
	if n := len(e.G.Zone(state.ZExile, 1)); n != 1 {
		t.Fatalf("p1 exile holds %d cards, want 1", n)
	}
}

// TestCloakAndDaggerNoCreatureTargetOffersOnlyTheHand: electing zero for the
// up-to-one creature target leaves the hand's nonland card the only candidate.
func TestCloakAndDaggerNoCreatureTargetOffersOnlyTheHand(t *testing.T) {
	t.Parallel()
	_, run, d := cloakDaggerRun(t, []string{"Lightning Bolt", "Forest"}, false)
	bears, _ := run.resolve("p1:Grizzly Bears")
	bolt, _ := run.resolve("p1:Lightning Bolt")
	if indexOfObjOption(d, bears) >= 0 || indexOfObjOption(d, bolt) < 0 || len(d.Options) != 1 {
		t.Fatalf("exile choice = %+v, want only the hand's Lightning Bolt", d.Options)
	}
}
