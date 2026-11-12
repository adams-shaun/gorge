package rules

// The self-carried S:Mode$ AlternativeCost | EffectZone$ Graveyard cast from
// the graveyard: Worldheart Phoenix's and Bringer of the Red Dawn's
// EffectZone$ All cousin must stay out, and the three already-supported
// corpus carriers (Raffine's Guidance, Squee, Scourge of Nel Toth) must be
// offered with their real costs paid.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// hasAltCostOption reports whether opts holds a "cast" option for id at the
// given AltCostIndex (0 is the printed cost, i+1 the list index into
// alternativeCosts).
func hasAltCostOption(opts []decision.Option, id state.ObjID, alt int) bool {
	for _, op := range opts {
		if op.Kind == "cast" && op.Obj == id && op.AltCostIndex == alt {
			return true
		}
	}
	return false
}

// hasAnyCastOption reports whether opts holds any "cast" option for id.
func hasAnyCastOption(opts []decision.Option, id state.ObjID) bool {
	for _, op := range opts {
		if op.Kind == "cast" && op.Obj == id {
			return true
		}
	}
	return false
}

// TestGraveyardAltCostManaFloor: the {W}{U}{B}{R}{G} alternative is the ONLY
// graveyard route; with RRRR the printed {3}{R} is not offered in its place.
func TestGraveyardAltCostManaFloor(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Worldheart Phoenix")}, nil)
	id := moveByName(t, e, 0, "Worldheart Phoenix", state.ZGraveyard)
	addMana(t, e, 0, "RRRR")
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: Phoenix zone=%s, want graveyard", z)
	}
	if hasAnyCastOption(e.Pending().Options, id) {
		t.Fatalf("Phoenix offered from the graveyard with RRRR: %+v", e.Pending().Options)
	}
}

// TestGraveyardAltCostHandStillUsesPrintedCost: in hand the EffectZone$
// Graveyard static does not apply (the hand offer is index 0 only), pinning
// the zone gate against the new graveyardSelf bit.
func TestGraveyardAltCostHandStillUsesPrintedCost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Worldheart Phoenix")}, nil)
	id := moveByName(t, e, 0, "Worldheart Phoenix", state.ZHand)
	addMana(t, e, 0, "WUBRG")
	if z := e.G.Obj(id).Zone; z != state.ZHand {
		t.Fatalf("precondition: Phoenix zone=%s, want hand", z)
	}
	if hasAltCostOption(e.Pending().Options, id, 1) {
		t.Fatalf("in-hand Phoenix offered the graveyard alternative: %+v", e.Pending().Options)
	}
	if !hasAltCostOption(e.Pending().Options, id, 0) {
		t.Fatalf("in-hand Phoenix not offered its printed cost: %+v", e.Pending().Options)
	}
}

// TestGraveyardAltCostEffectZoneAllNotWidened: Verdant Mastery's static is
// EffectZone$ All; effectZoneOK admits the graveyard for it, but it is not a
// graveyard cast and must not be offered from there.
func TestGraveyardAltCostEffectZoneAllNotWidened(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Verdant Mastery")}, nil)
	id := moveByName(t, e, 0, "Verdant Mastery", state.ZGraveyard)
	addMana(t, e, 0, "GGGG")
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: Verdant Mastery zone=%s, want graveyard", z)
	}
	if hasAnyCastOption(e.Pending().Options, id) {
		t.Fatalf("EffectZone$ All card offered from the graveyard: %+v", e.Pending().Options)
	}
}

// TestGraveyardAltCostRaffinesGuidance: an Aura cast from the graveyard for
// its {2}{W} alternative, with the creature it enchants already on the
// battlefield (the real precondition for the target-bearing offer).
func TestGraveyardAltCostRaffinesGuidance(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Raffine's Guidance"),
		lookup(t, reg, "Grizzly Bears")}, nil)
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	id := moveByName(t, e, 0, "Raffine's Guidance", state.ZGraveyard)
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: Guidance zone=%s, want graveyard", z)
	}
	if z := e.G.Obj(bears).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: Grizzly Bears zone=%s, want battlefield", z)
	}
	addMana(t, e, 0, "WWW")
	if !hasAltCostOption(e.Pending().Options, id, 1) {
		t.Fatalf("Raffine's Guidance not offered from the graveyard: %+v", e.Pending().Options)
	}
	castWithAltIndex(t, e, id, 1)
	answerKTarget(t, e, bears)
	passUntilStackEmpty(t, e, 20)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("cast Guidance zone=%s, want battlefield", o.Zone)
	}
	if o.AttachedTo != bears {
		t.Fatalf("Guidance attached to %d, want Grizzly Bears %d", o.AttachedTo, bears)
	}
	replayCheck(t, e, cfg)
}

// TestGraveyardAltCostSqueeExileCost: Squee's {3}{R} plus
// ExileFromGrave<4/Card.Other>. The casting card is not its own fodder: four
// OTHER cards are needed, and exactly those four are exiled on payment.
func TestGraveyardAltCostSqueeExileCost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	t.Run("four other cards are enough", func(t *testing.T) {
		e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Squee, Dubious Monarch")}, nil)
		id := moveByName(t, e, 0, "Squee, Dubious Monarch", state.ZGraveyard)
		fodder := make([]state.ObjID, 0, 4)
		for i := 0; i < 4; i++ {
			fodder = append(fodder, moveByName(t, e, 0, "Mountain", state.ZGraveyard))
		}
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("precondition: Squee zone=%s, want graveyard", z)
		}
		if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 5 {
			t.Fatalf("precondition: graveyard holds %d cards, want Squee + 4 others", got)
		}
		addMana(t, e, 0, "RRRR")
		if !hasAltCostOption(e.Pending().Options, id, 1) {
			t.Fatalf("Squee not offered with four other graveyard cards: %+v", e.Pending().Options)
		}
		castWithAltIndex(t, e, id, 1)
		drainAnsweringFirst(t, e, 30)
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			t.Fatalf("cast Squee zone=%s, want battlefield", z)
		}
		if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 0 {
			t.Fatalf("graveyard still holds %d cards after Squee's cost, want 0", got)
		}
		for _, f := range fodder {
			if z := e.G.Obj(f).Zone; z != state.ZExile {
				t.Fatalf("fodder %d zone=%s, want exile (four other cards, not Squee)", f, z)
			}
		}
		if got := len(e.G.Zone(state.ZExile, 0)); got != 4 {
			t.Fatalf("exile holds %d cards, want the four other cards", got)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("three other cards are not enough", func(t *testing.T) {
		e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Squee, Dubious Monarch")}, nil)
		id := moveByName(t, e, 0, "Squee, Dubious Monarch", state.ZGraveyard)
		for i := 0; i < 3; i++ {
			moveByName(t, e, 0, "Mountain", state.ZGraveyard)
		}
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("precondition: Squee zone=%s, want graveyard", z)
		}
		if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 4 {
			t.Fatalf("precondition: graveyard holds %d cards, want Squee + 3 others", got)
		}
		addMana(t, e, 0, "RRRR")
		if hasAnyCastOption(e.Pending().Options, id) {
			t.Fatalf("Squee offered with only three other cards: %+v", e.Pending().Options)
		}
	})
}

// TestGraveyardAltCostScourgeSacrifice: Scourge's {B}{B} plus Sac<2/Creature>;
// two creatures on the battlefield are needed, one is not.
func TestGraveyardAltCostScourgeSacrifice(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	t.Run("two creatures are enough", func(t *testing.T) {
		e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Scourge of Nel Toth"),
			lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Goblin Piker")}, nil)
		id := moveByName(t, e, 0, "Scourge of Nel Toth", state.ZGraveyard)
		bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
		piker := moveByName(t, e, 0, "Goblin Piker", state.ZBattlefield)
		if e.G.Obj(bears).Zone != state.ZBattlefield || e.G.Obj(piker).Zone != state.ZBattlefield {
			t.Fatalf("precondition: the two sacrifice creatures are not both on the battlefield")
		}
		addMana(t, e, 0, "BB")
		if !hasAltCostOption(e.Pending().Options, id, 1) {
			t.Fatalf("Scourge not offered with two creatures: %+v", e.Pending().Options)
		}
		castWithAltIndex(t, e, id, 1)
		drainAnsweringFirst(t, e, 30)
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			t.Fatalf("cast Scourge zone=%s, want battlefield", z)
		}
		for _, c := range []state.ObjID{bears, piker} {
			if z := e.G.Obj(c).Zone; z != state.ZGraveyard {
				t.Fatalf("sacrificed creature %d zone=%s, want graveyard", c, z)
			}
		}
		replayCheck(t, e, cfg)
	})
	t.Run("one creature is not enough", func(t *testing.T) {
		e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Scourge of Nel Toth"),
			lookup(t, reg, "Grizzly Bears")}, nil)
		id := moveByName(t, e, 0, "Scourge of Nel Toth", state.ZGraveyard)
		moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
		addMana(t, e, 0, "BB")
		if hasAnyCastOption(e.Pending().Options, id) {
			t.Fatalf("Scourge offered with one creature: %+v", e.Pending().Options)
		}
	})
}

// TestGraveyardAltCostQuietBoard: a Phoenix alone in the graveyard is a
// quiet-proof blocker, so the walk is not skipped and the offer survives the
// quiet entry point.
func TestGraveyardAltCostQuietBoard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := quietBaseWith(t, reg, []*cards.Card{lookup(t, reg, "Worldheart Phoenix")})
	id := addZone(t, e, 0, lookup(t, reg, "Worldheart Phoenix"), state.ZGraveyard)
	addMana(t, e, 0, "WUBRG")
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 1 {
		t.Fatalf("precondition: graveyard holds %d cards, want the Phoenix alone", got)
	}
	if e.seatQuiet(0) {
		t.Fatalf("quiet proof skipped the walk with a graveyard-self alternative in the graveyard (blocker %s)", quietBlockerNames[e.quietBlocker(0)])
	}
	if got := e.quietBlocker(0); got != qbGraveRoute {
		t.Fatalf("quiet blocker=%s, want graveyard route", quietBlockerNames[got])
	}
	if opts := e.legalActions(0); !hasAltCostOption(opts, id, 1) {
		t.Fatalf("Phoenix graveyard cast missing through the quiet entry point: %+v", opts)
	}
}
