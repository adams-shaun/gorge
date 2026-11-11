package rules

// Count$AltCost.<paid>.<unpaid> (Forge's AltCost head): <paid> when the cast
// paid an S:Mode$ AlternativeCost static's alternative. beginCast records the
// chosen alternative on the pending cast (altCostPaid, gated on the face
// mentioning Count$AltCost) and the pay-time CastInfo stamps
// state.FlagAltCostPaid; the announce-time asks read the pending bit
// (Ctx.Kicker.PendingAltCost). Every card here is the REAL corpus script.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// castWithAltIndex submits the cast option for id whose AltCostIndex is alt.
func castWithAltIndex(t *testing.T, e *Engine, id state.ObjID, alt int) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision pending (got %+v)", d)
	}
	for _, op := range d.Options {
		if op.Kind == "cast" && op.Obj == id && op.AltCostIndex == alt {
			submitChoices(t, e, op.Index)
			return
		}
	}
	t.Fatalf("precondition: no cast option for %d with AltCostIndex %d in %+v", id, alt, d.Options)
}

// altCostFlagEvents counts the CastInfo events naming altcostpaid (on obj, or
// on any object when obj is 0).
func altCostFlagEvents(e *Engine, obj state.ObjID) int {
	return countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.CastInfo && (obj == 0 || ev.Obj == obj) &&
			events.FlagsFrom(ev.Counter)&state.FlagAltCostPaid != 0
	})
}

func TestBalefulMasteryAltCostBranch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name     string
		alt      int
		oppDraws int
	}{
		{"alternative {1}{B}: an opponent draws", 1, 1},
		{"mana cost {3}{B}: nobody draws", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Baleful Mastery")},
				[]*cards.Card{lookup(t, reg, "Grizzly Bears")})
			bears := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
			id := moveByName(t, e, 0, "Baleful Mastery", state.ZHand)
			addMana(t, e, 0, "RRRB")
			opp := len(e.G.Zone(state.ZHand, 1))
			if len(e.G.Zone(state.ZLibrary, 1)) == 0 {
				t.Fatal("precondition: seat 1 has no library to draw from")
			}
			castWithAltIndex(t, e, id, tc.alt)
			d := e.Pending()
			if d == nil || d.Kind != decision.KTarget {
				t.Fatalf("precondition: want the target ask, got %+v", d)
			}
			pick := -1
			for _, op := range d.Options {
				if op.Obj == bears {
					pick = op.Index
				}
			}
			if pick < 0 {
				t.Fatalf("precondition: Grizzly Bears not a legal target: %+v", d.Options)
			}
			submitChoices(t, e, pick)
			drainAnsweringFirst(t, e, 30)
			if z := e.G.Obj(bears).Zone; z != state.ZExile {
				t.Fatalf("Grizzly Bears zone=%s, want exile", z)
			}
			if got := len(e.G.Zone(state.ZHand, 1)); got != opp+tc.oppDraws {
				t.Fatalf("seat 1 hand = %d, want %d", got, opp+tc.oppDraws)
			}
			if got, want := altCostFlagEvents(e, id) > 0, tc.alt == 1; got != want {
				t.Fatalf("altcostpaid CastInfo on Baleful Mastery = %v, want %v", got, want)
			}
			replayCheck(t, e, cfg)
		})
	}
	t.Run("a card without Count$AltCost stamps nothing", func(t *testing.T) {
		e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Lightning Bolt")}, nil)
		id := moveByName(t, e, 0, "Lightning Bolt", state.ZHand)
		addMana(t, e, 0, "R")
		castWithAltIndex(t, e, id, 0)
		answerTargetPlayer(t, e, 1)
		passUntilStackEmpty(t, e, 20)
		if n := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.CastInfo && ev.Obj == id }); n == 0 &&
			e.G.Obj(id).Zone != state.ZGraveyard {
			t.Fatalf("precondition: Lightning Bolt did not resolve (zone %s)", e.G.Obj(id).Zone)
		}
		if n := altCostFlagEvents(e, 0); n != 0 {
			t.Fatalf("%d CastInfo events carry altcostpaid in a game with no Count$AltCost card", n)
		}
	})
	t.Run("registry", func(t *testing.T) {
		for _, name := range []string{"Baleful Mastery", "Devastating Mastery", "Fervent Mastery",
			"Ingenious Mastery", "Verdant Mastery", "Graveyard Dig"} {
			if got := reg.Unsupported(lookup(t, reg, name), effects.Supported()); len(got) != 0 {
				t.Errorf("%s unsupported: %v", name, got)
			}
		}
	})
}

// graveyardDigTargetPool casts Graveyard Dig at alternative alt and returns
// every card offered by its target asks, answering each ask with nothing.
func graveyardDigTargetPool(t *testing.T, reg *cards.Registry, alt int) (offered map[string]bool, red, bears state.ObjID) {
	t.Helper()
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Graveyard Dig"),
		lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Goblin Piker")}, nil)
	bears = moveByName(t, e, 0, "Grizzly Bears", state.ZGraveyard)
	red = moveByName(t, e, 0, "Goblin Piker", state.ZGraveyard)
	if f := e.G.Obj(red).Face(); strings.ContainsAny(f.ManaCost, "BG") || strings.ContainsAny(f.Colors, "bg") {
		t.Fatalf("precondition: Goblin Piker must be neither black nor green (cost %q)", f.ManaCost)
	}
	id := moveByName(t, e, 0, "Graveyard Dig", state.ZHand)
	addMana(t, e, 0, "GGGG")
	castWithAltIndex(t, e, id, alt)
	offered = map[string]bool{}
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		if d.Kind != decision.KTarget {
			// A hybrid {B/G} payment ask: any colour pays.
			submitChoices(t, e, d.Options[0].Index)
			continue
		}
		for _, op := range d.Options {
			if op.Obj != 0 && d.Max > 0 {
				offered[e.G.Obj(op.Obj).Face().Name] = true
			}
		}
		submitChoices(t, e)
	}
	return offered, red, bears
}

func TestGraveyardDigAltCostTargetBounds(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	alt, _, _ := graveyardDigTargetPool(t, reg, 1)
	plain, _, _ := graveyardDigTargetPool(t, reg, 0)
	if !plain["Grizzly Bears"] || plain["Goblin Piker"] {
		t.Fatalf("plain cast offered %v, want the green Grizzly Bears and not the red Goblin Piker", plain)
	}
	if !alt["Grizzly Bears"] || !alt["Goblin Piker"] {
		t.Fatalf("alternative cast offered %v, want any creature (Goblin Piker included)", alt)
	}
}

func TestWorldheartPhoenixAltCostCounters(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	t.Run("hand cast enters without counters", func(t *testing.T) {
		e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Worldheart Phoenix")}, nil)
		id := moveByName(t, e, 0, "Worldheart Phoenix", state.ZHand)
		addMana(t, e, 0, "RRRR")
		castWithAltIndex(t, e, id, 0)
		passUntilStackEmpty(t, e, 20)
		o := e.G.Obj(id)
		if o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Phoenix zone=%s, want battlefield", o.Zone)
		}
		if got := o.Counter("P1P1"); got != 0 {
			t.Fatalf("hand-cast Phoenix P1P1=%d, want 0", got)
		}
		replayCheck(t, e, cfg)
	})
	// The {W}{U}{B}{R}{G} self-carried graveyard cast (EffectZone$ Graveyard):
	// offered through graveyardAltCostCasts, paid through the full-list
	// AltCostIndex, and the AltCostPaid-gated replacement gives the counters.
	t.Run("graveyard cast enters with two counters", func(t *testing.T) {
		e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Worldheart Phoenix")}, nil)
		id := moveByName(t, e, 0, "Worldheart Phoenix", state.ZGraveyard)
		addMana(t, e, 0, "WUBRG")
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("precondition: Phoenix zone=%s, want graveyard", z)
		}
		if !hasAltCostOption(e.Pending().Options, id, 1) {
			t.Fatalf("no graveyard cast option with AltCostIndex 1 in %+v", e.Pending().Options)
		}
		if hasAltCostOption(e.Pending().Options, id, 0) {
			t.Fatalf("the printed {3}{R} cost is offered from the graveyard: %+v", e.Pending().Options)
		}
		castWithAltIndex(t, e, id, 1)
		passUntilStackEmpty(t, e, 20)
		o := e.G.Obj(id)
		if o.Zone != state.ZBattlefield {
			t.Fatalf("graveyard-cast Phoenix zone=%s, want battlefield", o.Zone)
		}
		if got := o.Counter("P1P1"); got != 2 {
			t.Fatalf("graveyard-cast Phoenix P1P1=%d, want 2", got)
		}
		if altCostFlagEvents(e, id) == 0 {
			t.Fatal("no altcostpaid CastInfo event on the graveyard cast")
		}
		replayCheck(t, e, cfg)
	})
}

// drainAnsweringFirst passes priority until the stack is empty, answering any
// other ask with its first Min options.
func drainAnsweringFirst(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for n := 0; n < limit && !e.G.Over && len(e.G.Stack) > 0; n++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the stack")
		}
		var pick []int
		if d.Kind == decision.KPriority {
			for _, op := range d.Options {
				if op.Kind == "pass" {
					pick = []int{op.Index}
				}
			}
		} else {
			for i := 0; i < d.Min && i < len(d.Options); i++ {
				pick = append(pick, d.Options[i].Index)
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: pick}); err != nil {
			t.Fatalf("drain submit %v to %v %q: %v", pick, d.Kind, d.ResumeKind, err)
		}
	}
	if len(e.G.Stack) > 0 {
		t.Fatalf("stack not empty after %d answers", limit)
	}
}

// TestBringerOfGreenZenithsTwilightAltCost: the registry flip this head makes
// for Bringer of Green Zenith's Twilight (Count$AltCost.5.X, an SVar operand)
// is real: the {W}{U}{B}{R}{G} cast reads X as 5 and enters with twice that
// (10 counters); the {X}{G}{G} cast at X=2 enters with 2.
func TestBringerOfGreenZenithsTwilightAltCost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name string
		alt  int
		mana string
		want int32
	}{
		{"alternative WUBRG", 1, "WUBRG", 10},
		{"mana cost at X=2", 0, "GGGG", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Bringer of Green Zenith's Twilight")}, nil)
			id := moveByName(t, e, 0, "Bringer of Green Zenith's Twilight", state.ZHand)
			addMana(t, e, 0, tc.mana)
			castWithAltIndex(t, e, id, tc.alt)
			if tc.alt == 0 {
				announceXCast(t, e, id, 2)
			}
			drainAnsweringFirst(t, e, 20)
			o := e.G.Obj(id)
			if o.Zone != state.ZBattlefield {
				t.Fatalf("precondition: Bringer zone=%s, want battlefield", o.Zone)
			}
			if got := o.Counter("P1P1"); got != tc.want {
				t.Fatalf("Bringer P1P1=%d, want %d", got, tc.want)
			}
			replayCheck(t, e, cfg)
		})
	}
}
