// cr-g52: CR 903.3b/903.3c. A melded permanent whose cards include a
// player's commander, and a merged (mutated) permanent with a commander
// among its components, IS that player's commander: its combat damage is
// commander damage (CR 903.10a), and when it leaves the battlefield the
// commander card that goes to the graveyard or exile is offered the command
// zone (CR 903.9a). Mutate fixtures are authored (never Forge text); the meld
// pair is the real corpus Gisela / Bruna.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// mutateCmdSrc: a legendary non-Human {2}{G} 3/3 commander with Mutate {1}{G}.
const mutateCmdSrc = "Name:Apex Tester\nManaCost:2 G\nTypes:Legendary Creature Beast\nPT:3/3\nK:Mutate:1 G\nOracle:x\n"

// mutateRiderSrc: a non-legendary {G} 1/1 with Mutate {G}, mutated onto the
// commander in the guard test.
const mutateRiderSrc = "Name:Pile Rider\nManaCost:G\nTypes:Creature Beast\nPT:1/1\nK:Mutate:G\nOracle:x\n"

// mutatedCommanderGame casts seat 0's mutate commander from the command zone
// onto a Mutate Bear seat 0 controls, on top or under, and returns the
// engine, the commander's original object and the pile (the Bear's object).
func mutatedCommanderGame(t *testing.T, top bool) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e, _ := cmdZoneGame(t, [][]string{{mutateCmdSrc}, {}})
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	cmd := e.G.Zone(state.ZCommand, 0)[0]
	bear := onBoardCard(t, e, 0, card(t, mutateBearSrc))
	addMana(t, e, 0, "GG")
	mutateCastOnto(t, e, mutatedCastOption(t, e, cmd), bear, top)
	if p := e.G.Obj(bear); p == nil || p.Zone != state.ZBattlefield || len(p.MergedCards) != 1 {
		t.Fatalf("top=%v: the mutation did not merge onto the Bear: %+v", top, p)
	}
	return e, cmd, bear
}

// askedCommanderCard dies the permanent id (a logged battlefield ->
// graveyard move), runs the state-based check, and returns the pending
// CR 903.9a ask's source and the name of the card that source object is.
func askedCommanderCard(t *testing.T, e *Engine, id state.ObjID) (state.ObjID, string) {
	t.Helper()
	e.pending = nil // the post-combat priority round; the SBA poses nothing behind a pending ask
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	e.checkStateBased()
	d := e.Pending()
	if d == nil || d.Kind != decision.KCommanderZone {
		t.Fatalf("CR 903.9a: after the permanent died, pending = %+v, want seat 0's commander_zone ask", d)
	}
	o := e.G.Obj(d.Source)
	if o == nil || o.Zone != state.ZGraveyard || o.Face() == nil {
		t.Fatalf("ask source %d is not a graveyard card: %+v", d.Source, o)
	}
	return d.Source, o.Face().Name
}

// TestCR903_3cMutatedCommanderStaysTheCommander: the commander mutated onto a
// Bear, on top and under. The pile's combat damage is commander damage, and
// when the pile dies the commander card is asked about; accepting moves the
// commander card (only) to the command zone.
func TestCR903_3cMutatedCommanderStaysTheCommander(t *testing.T) {
	t.Parallel()
	for _, top := range []bool{true, false} {
		e, _, pile := mutatedCommanderGame(t, top)
		swing(t, e, pile)
		if got := e.G.Players[1].CmdDamage[0]; got != e.Power(pile) || got == 0 {
			t.Errorf("top=%v: commander damage after the pile hit = %d, want its power %d (CR 903.3c, 903.10a)", top, got, e.Power(pile))
		}
		src, name := askedCommanderCard(t, e, pile)
		if name != "Apex Tester" {
			t.Fatalf("top=%v: CR 903.9a asked about %q, want the commander card Apex Tester", top, name)
		}
		submitChoices(t, e, 0)
		if got := e.G.Obj(src).Zone; got != state.ZCommand {
			t.Fatalf("top=%v: accepted commander card zone = %v, want command", top, got)
		}
		if got := e.G.Zone(state.ZCommand, 0); len(got) != 1 || e.G.Obj(got[0]).Face().Name != "Apex Tester" {
			t.Fatalf("top=%v: command zone = %v, want exactly Apex Tester", top, got)
		}
		if bear := e.G.Zone(state.ZGraveyard, 0); len(bear) != 1 || e.G.Obj(bear[0]).Face().Name != "Mutate Bear" {
			t.Fatalf("top=%v: graveyard = %v, want exactly the Mutate Bear", top, bear)
		}
	}
}

// TestCR903_3cCreatureMutatedOntoTheCommanderKeepsIt is the guard for the
// other direction: a non-commander mutates ON TOP of the fielded commander.
// The pile is still the commander (damage tallies), and on death the ask
// names the commander card, never the rider that sat on top.
func TestCR903_3cCreatureMutatedOntoTheCommanderKeepsIt(t *testing.T) {
	t.Parallel()
	riderCard := card(t, mutateRiderSrc)
	deck0 := append([]*cards.Card{card(t, mutateCmdSrc), riderCard}, mountainDeck(t, 38)...)
	cfg := seatZeroStart(Config{Seed: commanderDamageSeed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck0, mountainDeck(t, 40)}, Commanders: [][]int{{0}, {}},
		StartingLife: 40, Format: FormatCommander})
	e := New(cfg)
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	cmd := fieldCommander(t, e, 0, 0)
	rider := moveSeededCard(t, e, 0, riderCard, state.ZHand)
	addMana(t, e, 0, "G")
	mutateCastOnto(t, e, mutatedCastOption(t, e, rider), cmd, true)
	if p := e.G.Obj(cmd); p == nil || p.Face().Name != "Pile Rider" {
		t.Fatalf("pile top = %+v, want the rider on top", p)
	}
	swing(t, e, cmd)
	if got := e.G.Players[1].CmdDamage[0]; got != e.Power(cmd) || got == 0 {
		t.Errorf("commander damage after the pile hit = %d, want its power %d", got, e.Power(cmd))
	}
	_, name := askedCommanderCard(t, e, cmd)
	if name != "Apex Tester" {
		t.Fatalf("CR 903.9a asked about %q, want the commander card Apex Tester", name)
	}
}

// TestCR903_3bMeldedCommanderIsTheCommander: seat 0's commander is Bruna,
// the Fading Light (the meld half WITHOUT the result face); it melds with
// Gisela into Brisela, which keeps Gisela's object. Brisela's combat damage
// is commander damage, and when Brisela dies Bruna is asked about.
func TestCR903_3bMeldedCommanderIsTheCommander(t *testing.T) {
	t.Parallel()
	e, _ := cmdZoneGame(t, [][]string{{corpusCardText(t, "b/bruna_the_fading_light.txt")}, {}})
	bruna := fieldCommander(t, e, 0, 0)
	gisela := onBoardCard(t, e, 0, corpusCard(t, "Gisela, the Broken Blade"))
	res, ok := effects.Meld(e, 0, gisela, bruna, state.MeldOptions{ResultName: "Brisela, Voice of Nightmares"})
	if !ok || res != gisela {
		t.Fatalf("Meld = %d,%v; want Gisela's object %d", res, ok, gisela)
	}
	swing(t, e, res)
	if got := e.G.Players[1].CmdDamage[0]; got != 9 {
		t.Errorf("commander damage after Brisela hit = %d, want 9 (CR 903.3b, 903.10a)", got)
	}
	src, name := askedCommanderCard(t, e, res)
	if src != bruna || name != "Bruna, the Fading Light" {
		t.Fatalf("CR 903.9a asked about %d %q, want Bruna (%d)", src, name, bruna)
	}
}
