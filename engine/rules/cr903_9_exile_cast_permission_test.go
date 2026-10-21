// cr-g49: CR 903.9a's command-zone ask for a commander that went to exile
// with a standing cast-from-exile permission (an Adventure's CR 715.3d
// return, a foretold card's CR 702.143a later cast, a plotted card's CR
// 702.170 free cast). Moving it to the command zone ends that permission
// (CR 400.7) and makes the next cast pay the CR 903.8 tax, so the "leave"
// option names the permission in its Mode -- the same Mode string the
// exile walk later offers the cast under -- for a client or bot to read.
// Authored fixtures only -- never Forge text.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// adventureCmdSrc: a legendary {1}{W} 2/2 over a {G} Sorcery Adventure.
const adventureCmdSrc = "Name:Trail Warden\nManaCost:1 W\nTypes:Legendary Creature Human Scout\nPT:2/2\n" +
	"AlternateMode:Adventure\nOracle:front\nALTERNATE\nName:Trail Mix\nManaCost:G\n" +
	"Types:Sorcery Adventure\nA:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\nOracle:back\n"

// plotCmdSrc: a legendary {2}{R} 3/3 with Plot {R}.
const plotCmdSrc = "Name:Dust Drifter\nManaCost:2 R\nTypes:Legendary Creature Human Rogue\nPT:3/3\n" +
	"K:Plot:R\nOracle:x\n"

// foretellCmdSrc: a legendary {3}{R} 4/4 with Foretell {1}{R}.
const foretellCmdSrc = "Name:Omen Reader\nManaCost:3 R\nTypes:Legendary Creature Human Shaman\nPT:4/4\n" +
	"K:Foretell:1 R\nOracle:x\n"

// exiledCommanderAsk deals seat 0 the given commander, moves it from the
// command zone to its owner's hand (declining the CR 903.9b ask), funds
// mana, takes the hand cast option whose Mode is mode, and drives the
// result until the CR 903.9a ask for the now-exiled commander is pending.
func exiledCommanderAsk(t *testing.T, src, mana, mode string) (*Engine, state.ObjID, *decision.Decision) {
	t.Helper()
	e, _ := cmdZoneGame(t, [][]string{{src}, {}})
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	cmd := e.G.Zone(state.ZCommand, 0)[0]
	e.pending = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: cmd, From: state.ZCommand, To: state.ZHand})
	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone {
		t.Fatalf("hand move: pending = %+v, want the CR 903.9b ask", d)
	}
	submitChoices(t, e, 1) // leave it in hand
	addMana(t, e, 0, mana)
	var opt *decision.Option
	for _, o := range castOptions(t, e) {
		if o.Obj == cmd && o.Mode == mode {
			o := o
			opt = &o
		}
	}
	if opt == nil {
		t.Fatalf("no %q option for the commander in hand: %+v", mode, castOptions(t, e))
	}
	submitChoices(t, e, opt.Index)
	d := passUntilResolved(t, e, 20)
	if got := e.G.Obj(cmd).Zone; got != state.ZExile {
		t.Fatalf("%s: commander zone = %v, want exile", mode, got)
	}
	if d == nil || d.Kind != decision.KCommanderZone || d.Source != cmd {
		t.Fatalf("%s: pending = %+v, want the CR 903.9a ask for the exiled commander", mode, d)
	}
	return e, cmd, d
}

// TestCR903_9aExiledCommanderLeaveNamesItsCastPermission: each of the three
// standing exile permissions is named on the "leave" option's Mode, and the
// "command_zone" option carries none.
func TestCR903_9aExiledCommanderLeaveNamesItsCastPermission(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, src, mana, mode, want string }{
		{"adventure", adventureCmdSrc, "G", "adventure_alt", "adventure_recast"},
		{"plot", plotCmdSrc, "R", "plot", "plot_cast"},
		{"foretell", foretellCmdSrc, "CC", "foretell", "foretell_cast"},
	} {
		_, _, d := exiledCommanderAsk(t, tc.src, tc.mana, tc.mode)
		if len(d.Options) != 2 || d.Options[0].Kind != "command_zone" || d.Options[1].Kind != "leave" {
			t.Fatalf("%s: options = %+v, want [command_zone, leave]", tc.name, d.Options)
		}
		if got := d.Options[1].Mode; got != tc.want {
			t.Errorf("%s: leave Mode = %q, want %q (the exile cast permission it keeps)", tc.name, got, tc.want)
		}
		if got := d.Options[0].Mode; got != "" {
			t.Errorf("%s: command_zone Mode = %q, want empty", tc.name, got)
		}
	}
}

// TestCR903_9aExiledCommanderWithoutPermissionLeaveHasNoMode is the guard:
// a commander exiled with no cast permission (Dauthi Voidwalker's
// graveyard-bound replacement) keeps the plain "leave" option.
func TestCR903_9aExiledCommanderWithoutPermissionLeaveHasNoMode(t *testing.T) {
	t.Parallel()
	e, _ := cmdZoneGame(t, [][]string{{tinyCmdSrc}, {}})
	cmd := fieldCommander(t, e, 0, 0)
	onBoardCard(t, e, 1, corpusCard(t, "Dauthi Voidwalker"))
	e.G.Obj(cmd).Damage = 1
	e.checkStateBased()
	d := e.Pending()
	if d == nil || d.Kind != decision.KCommanderZone || e.G.Obj(cmd).Zone != state.ZExile {
		t.Fatalf("pending = %+v (zone %v), want the CR 903.9a ask for the exiled commander", d, e.G.Obj(cmd).Zone)
	}
	if got := d.Options[1].Mode; got != "" {
		t.Fatalf("leave Mode = %q, want empty (no exile cast permission)", got)
	}
}

// TestCR903_9aLeftExiledAdventureCommanderIsCastFromExileUntaxed pins why
// the Mode matters: declining keeps the Adventure return, and the creature
// is offered from exile with no CR 903.8 tax.
func TestCR903_9aLeftExiledAdventureCommanderIsCastFromExileUntaxed(t *testing.T) {
	t.Parallel()
	e, cmd, _ := exiledCommanderAsk(t, adventureCmdSrc, "G", "adventure_alt")
	submitChoices(t, e, 1) // leave
	addMana(t, e, 0, "WW")
	if adventureOption(t, e, cmd, "adventure_recast") == nil {
		t.Fatalf("the left commander must be castable from exile for {1}{W}: %+v", castOptions(t, e))
	}
}
