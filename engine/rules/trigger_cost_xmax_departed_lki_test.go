package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// xmaxPowerProbeSrc is a Madame Null-shaped watcher whose payer-chosen {X}
// carries an XMax$ cap reading the entering creature's power ("X can't be
// greater than its power"), so the cap body sees the same TriggeredCard$
// referent a fixed X body does.
const xmaxPowerProbeSrc = "Name:XMax Power Probe\n" +
	"ManaCost:2 B\n" +
	"Types:Creature Advisor\n" +
	"PT:1/3\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other+YouCtrl | Execute$ TrigPut | TriggerZones$ Battlefield | TriggerDescription$ Whenever another creature you control enters, you may pay {X}. If you do, put X +1/+1 counters on it. X can't be greater than its power.\n" +
	"SVar:TrigPut:AB$ PutCounter | Cost$ X | XMax$ Pow | Defined$ TriggeredCardLKICopy | CounterType$ P1P1 | CounterNum$ X\n" +
	"SVar:X:Count$xPaid\n" +
	"SVar:Pow:TriggeredCard$CardPower\n" +
	"Oracle:x\n"

// TestTriggerCostXMaxReadsDepartedCardLastKnownPower: the XMax$ cap of a
// triggered-cost window binds the parked trigger's LKI record like the fixed X
// body does (CR 603.10, 608.2h). The anthem'd Bears are 3/3 as they are
// sacrificed and 2/2 in the graveyard, so the announced X options run 0..3,
// not 0..2.
func TestTriggerCostXMaxReadsDepartedCardLastKnownPower(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	probe, diags := cards.ParseBytes("xmax_power_probe.txt", []byte(xmaxPowerProbeSrc))
	if len(diags) != 0 {
		t.Fatalf("parse diags: %+v", diags)
	}
	if diags = probe.Link(); len(diags) != 0 {
		t.Fatalf("link diags: %+v", diags)
	}
	anthem, _ := reg.Lookup("Glorious Anthem")
	altar, _ := reg.Lookup("Ashnod's Altar")
	bears, _ := reg.Lookup("Grizzly Bears")
	deck := []*cards.Card{probe, anthem, altar, bears}
	deck = append(deck, mountainDeck(t, 40-len(deck))...)
	cfg := seatZeroStart(Config{Seed: 209, Names: []string{"probe", "opponent"},
		Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	probeID := moveByName(t, e, 0, "XMax Power Probe", state.ZBattlefield)
	for _, name := range []string{"Glorious Anthem", "Ashnod's Altar"} {
		moveByName(t, e, 0, name, state.ZBattlefield)
	}
	bearsID := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	addMana(t, e, 0, "CGGGGG")

	d := e.Pending()
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == bearsID {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("no cast option for the Bears: %+v", d.Options)
	}
	submitChoices(t, e, castIdx)
	passPriority(t, e)
	passPriority(t, e)
	if !onBattlefield(e, bearsID) || !onBattlefield(e, probeID) {
		t.Fatal("precondition: the Bears or the probe is not on the battlefield")
	}
	if got := e.Power(bearsID); got != 3 {
		t.Fatalf("precondition: Bears power on the battlefield = %d, want 3 (anthem applied)", got)
	}

	altarID := findBattlefield(t, e, 0, "Ashnod's Altar", 0)
	d = e.Pending()
	altIdx := -1
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == altarID {
			altIdx = o.Index
		}
	}
	if altIdx < 0 {
		t.Fatalf("no activation option for Ashnod's Altar: %+v", d.Options)
	}
	submitChoices(t, e, altIdx)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		chooseObjOption(t, e, d, bearsID)
	}
	if o := e.G.Obj(bearsID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Bears zone = %+v, want the graveyard", o)
	}
	d = passUntilNonPriority(t, e, 40)
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "trigger_cost_x" {
		t.Fatalf("precondition: no X announcement ask: %+v", d)
	}
	max := -1
	for _, o := range d.Options {
		if o.Amount > max {
			max = o.Amount
		}
	}
	if max != 3 {
		t.Fatalf("largest announceable X = %d, want 3 (the Bears' last-known battlefield power; the graveyard face would cap at 2)", max)
	}
	replayCheck(t, e, cfg)
}
