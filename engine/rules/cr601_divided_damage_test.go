package rules

// CR 601.2d (and 602.2b for activations): a spell that divides damage among
// its targets announces the division WHILE IT IS CAST, after the targets
// (601.2c) and before costs are paid, and each target receives at least 1.
// On resolution a target that has become illegal is not dealt its share and
// the share is NOT redistributed (CR 608.2b: illegal targets are unaffected).
// Real corpus cards: Arc Lightning ("3 damage divided as you choose among
// one, two, or three targets") and Grizzly Bears.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// arcSetup puts Arc Lightning in seat 0's hand with RRR floating and `bears`
// corpus Grizzly Bears on seat 1's battlefield, then casts Arc Lightning and
// returns the pending KTarget decision.
func arcSetup(t *testing.T, bears int) (*Engine, Config, state.ObjID, []state.ObjID, *decision.Decision) {
	t.Helper()
	reg := choiceCorpusRegistry(t)
	var opp []*cards.Card
	for i := 0; i < bears; i++ {
		opp = append(opp, choiceCorpusCard(t, "Grizzly Bears"))
	}
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{choiceCorpusCard(t, "Arc Lightning")}, opp)
	if e.G.Active != 0 {
		t.Fatalf("precondition: seat 0 must be active to cast a sorcery, active = %d", e.G.Active)
	}
	arc := moveByName(t, e, 0, "Arc Lightning", state.ZHand)
	var ids []state.ObjID
	for i := 0; i < bears; i++ {
		ids = append(ids, moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield))
	}
	addMana(t, e, 0, "RRR")
	submitChoices(t, e, castOptionFor(t, e, arc).Index)
	td := e.Pending()
	if td == nil || td.Kind != decision.KTarget {
		t.Fatalf("Arc Lightning's target ask missing: %+v", td)
	}
	if td.Min != 1 || td.Max != 3 {
		t.Fatalf("target bounds = [%d,%d], want [1,3]", td.Min, td.Max)
	}
	return e, cfg, arc, ids, td
}

func optForObj(t *testing.T, d *decision.Decision, obj state.ObjID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind != "player" && o.Obj == obj {
			return o.Index
		}
	}
	t.Fatalf("object %d not offered: %+v", obj, d.Options)
	return -1
}

func optForPlayer(t *testing.T, d *decision.Decision, p state.PlayerID) int {
	t.Helper()
	for _, o := range d.Options {
		if (o.Kind == "player" || (o.Kind == "divide" && o.Obj == 0)) && o.Player == p {
			return o.Index
		}
	}
	t.Fatalf("player %d not offered: %+v", p, d.Options)
	return -1
}

// isCastDivision reports whether d is the cast-time division announcement.
func isCastDivision(d *decision.Decision) bool {
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 {
		return false
	}
	for _, o := range d.Options {
		if o.Kind != "divide" {
			return false
		}
	}
	return true
}

func killBear(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: bear zone = %s after the response, want graveyard", z)
	}
}

// TestArcLightningAnnouncesDivisionWhileCasting: with two targets the caster
// announces how the 3 damage is divided during casting (CR 601.2d), BEFORE
// priority returns and before the spell resolves. Each target gets at least
// 1, so the only free choice is where the third point goes: one pick among
// the two targets (Min == Max == 1, Repeatable). The answer is recorded on
// the stack object by a Choose "divide" event.
func TestArcLightningAnnouncesDivisionWhileCasting(t *testing.T) {
	t.Parallel()
	e, cfg, arc, bears, td := arcSetup(t, 1)
	submitChoices(t, e, optForObj(t, td, bears[0]), optForPlayer(t, td, 1))
	dd := e.Pending()
	if !isCastDivision(dd) {
		t.Fatalf("CR 601.2d: no division announced while casting; pending after targets = %+v", dd)
	}
	if dd.Min != 1 || dd.Max != 1 || !dd.Repeatable || len(dd.Options) != 2 {
		t.Fatalf("division ask = min %d max %d repeatable %v options %d, want 1/1/true/2",
			dd.Min, dd.Max, dd.Repeatable, len(dd.Options))
	}
	if z := e.G.Obj(arc).Zone; z != state.ZStack {
		t.Fatalf("Arc Lightning zone = %s during the announcement, want stack", z)
	}
	submitChoices(t, e, optForObj(t, dd, bears[0]))
	if p := e.Pending(); p == nil || p.Kind != decision.KPriority || p.Player != 0 {
		t.Fatalf("after the division the caster must hold priority with the spell on the stack, got %+v", p)
	}
	n := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.Choose && ev.Obj == arc && ev.Counter == "divide"
	})
	if n != 1 {
		t.Fatalf("Choose \"divide\" events on Arc Lightning = %d, want 1", n)
	}
	replayCheck(t, e, cfg)
}

// TestArcLightningIllegalTargetLosesItsShare is the audit repro: 2 to the
// Bears and 1 to the opponent, the Bears die in response. CR 608.2b: the
// opponent takes exactly the 1 announced for them; the Bears' 2 is lost.
func TestArcLightningIllegalTargetLosesItsShare(t *testing.T) {
	t.Parallel()
	e, cfg, _, bears, td := arcSetup(t, 1)
	submitChoices(t, e, optForObj(t, td, bears[0]), optForPlayer(t, td, 1))
	if dd := e.Pending(); isCastDivision(dd) {
		submitChoices(t, e, optForObj(t, dd, bears[0])) // Bears 2, player 1
	}
	killBear(t, e, bears[0])
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[1].Life; got != 19 {
		t.Fatalf("opponent life = %d, want 19: the announced 1 only, the dead Bears' 2 must not be redistributed (CR 601.2d, 608.2b)", got)
	}
	replayCheck(t, e, cfg)
}

// TestArcLightningThreeTargetsKeepOneEach: three targets each receive exactly
// 1 (the only legal division, so nothing is asked). One Bears dies in
// response: the surviving Bears and the opponent still take 1 each.
func TestArcLightningThreeTargetsKeepOneEach(t *testing.T) {
	t.Parallel()
	e, cfg, _, bears, td := arcSetup(t, 2)
	submitChoices(t, e, optForObj(t, td, bears[0]), optForObj(t, td, bears[1]), optForPlayer(t, td, 1))
	if p := e.Pending(); p == nil || p.Kind != decision.KPriority {
		t.Fatalf("three targets for 3 damage have one legal division; nothing may be asked, got %+v", p)
	}
	killBear(t, e, bears[0])
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Obj(bears[1]).Damage; got != 1 || e.G.Obj(bears[1]).Zone != state.ZBattlefield {
		t.Fatalf("surviving Bears damage = %d zone = %s, want 1 on the battlefield", got, e.G.Obj(bears[1]).Zone)
	}
	if got := e.G.Players[1].Life; got != 19 {
		t.Fatalf("opponent life = %d, want 19", got)
	}
	replayCheck(t, e, cfg)
}

// TestArcLightningAllTargetsLegalDealsTheDivision is the guard that holds
// before and after the fix: with no response, the 2/1 division is dealt as
// chosen. It answers whichever division ask the build poses (the cast-time
// one, or the legacy resolution-time "damage_split").
func TestArcLightningAllTargetsLegalDealsTheDivision(t *testing.T) {
	t.Parallel()
	e, cfg, _, bears, td := arcSetup(t, 1)
	submitChoices(t, e, optForObj(t, td, bears[0]), optForPlayer(t, td, 1))
	for n := 0; n < 30 && !e.G.Over && len(e.G.Stack) > 0; n++ {
		d := e.Pending()
		switch {
		case isCastDivision(d):
			submitChoices(t, e, optForObj(t, d, bears[0]))
		case d != nil && d.Kind == decision.KChoose && d.ResumeKind == "damage_split":
			b, p := optForObj(t, d, bears[0]), optForPlayer(t, d, 1)
			submitChoices(t, e, b, b, p)
		default:
			passUntilStackEmpty(t, e, 1)
		}
	}
	if z := e.G.Obj(bears[0]).Zone; z != state.ZGraveyard {
		t.Fatalf("Bears zone = %s, want graveyard (dealt 2)", z)
	}
	if got := e.G.Players[1].Life; got != 19 {
		t.Fatalf("opponent life = %d, want 19 (dealt 1)", got)
	}
	replayCheck(t, e, cfg)
}

// TestMoggMobActivationAnnouncesDivision is the CR 602.2b twin: Mogg Mob's
// "Sacrifice Mogg Mob: 3 damage divided as you choose among one, two, or
// three targets" announces its division while it is activated. 2 to the
// Bears and 1 to the opponent; the Bears die in response; the opponent
// takes exactly 1.
func TestMoggMobActivationAnnouncesDivision(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{choiceCorpusCard(t, "Mogg Mob")},
		[]*cards.Card{choiceCorpusCard(t, "Grizzly Bears")})
	mob := moveByName(t, e, 0, "Mogg Mob", state.ZBattlefield)
	bear := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, mob, 0).Index)
	td := e.Pending()
	if td == nil || td.Kind != decision.KTarget {
		t.Fatalf("Mogg Mob's target ask missing: %+v", td)
	}
	submitChoices(t, e, optForObj(t, td, bear), optForPlayer(t, td, 1))
	dd := e.Pending()
	if !isCastDivision(dd) {
		t.Fatalf("CR 602.2b/601.2d: no division announced while activating; pending = %+v", dd)
	}
	submitChoices(t, e, optForObj(t, dd, bear)) // Bears 2, player 1
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v after the activation, want the one ability", e.G.Stack)
	}
	n := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.Choose && ev.Obj == e.G.Stack[0] && ev.Counter == "divide"
	})
	if n != 1 {
		t.Fatalf("Choose \"divide\" events on the ability = %d, want 1", n)
	}
	killBear(t, e, bear)
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[1].Life; got != 19 {
		t.Fatalf("opponent life = %d, want 19 (the announced 1 only)", got)
	}
	replayCheck(t, e, cfg)
}

// TestArcLightningCopyInheritsDivision: CR 707.10, a copy copies every
// decision made for the spell, the division included. Arc Lightning divides
// 1 to a Colossal Dreadmaw (6/6, so nothing dies) and 2 to the opponent;
// Twincast copies it keeping the targets. Copy and original each deal 1 + 2:
// the Dreadmaw has 2 damage and the opponent is at 16.
func TestArcLightningCopyInheritsDivision(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{choiceCorpusCard(t, "Arc Lightning"), choiceCorpusCard(t, "Twincast")},
		[]*cards.Card{choiceCorpusCard(t, "Colossal Dreadmaw")})
	arc := moveByName(t, e, 0, "Arc Lightning", state.ZHand)
	twin := moveByName(t, e, 0, "Twincast", state.ZHand)
	maw := moveByName(t, e, 1, "Colossal Dreadmaw", state.ZBattlefield)
	addMana(t, e, 0, "RRR")
	submitChoices(t, e, castOptionFor(t, e, arc).Index)
	td := e.Pending()
	submitChoices(t, e, optForObj(t, td, maw), optForPlayer(t, td, 1))
	dd := e.Pending()
	if !isCastDivision(dd) {
		t.Fatalf("CR 601.2d: no division announced while casting; pending = %+v", dd)
	}
	submitChoices(t, e, optForPlayer(t, dd, 1)) // Dreadmaw 1, player 2
	addMana(t, e, 0, "UU")
	submitChoices(t, e, castOptionFor(t, e, twin).Index)
	tt := e.Pending()
	if tt == nil || tt.Kind != decision.KTarget {
		t.Fatalf("Twincast's target ask missing: %+v", tt)
	}
	submitChoices(t, e, optForObj(t, tt, arc))
	for n := 0; n < 40 && !e.G.Over && len(e.G.Stack) > 0; n++ {
		d := e.Pending()
		if d != nil && d.Kind == decision.KTarget && d.ResumeKind == "copy_targets" {
			// Keep the copy's inherited targets (CR 707.10c lets them stay).
			submitChoices(t, e, optForObj(t, d, maw), optForPlayer(t, d, 1))
			continue
		}
		passUntilStackEmpty(t, e, 1)
	}
	if got := e.G.Obj(maw).Damage; got != 2 {
		t.Fatalf("Dreadmaw damage = %d, want 2 (1 from the copy's inherited division, 1 from the original)", got)
	}
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("opponent life = %d, want 16 (2 from the copy, 2 from the original)", got)
	}
	replayCheck(t, e, cfg)
}
