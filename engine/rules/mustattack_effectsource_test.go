package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Gideon Jura's +2 ("During target opponent's next turn, creatures that
// player controls attack Gideon Jura if able"): a DelayedTrigger remembers
// the opponent, its Execute$ Effect captures that player
// (RememberObjects$ DelayTriggerRemembered) and registers a MustAttack on the
// creatures the remembered player controls, naming the effect's source as
// what they must attack. The duty names the effect's SOURCE permanent
// (CR 508.1d), so only an attack on Gideon discharges it.
const gideonJuraPlusTwoToP1Upkeep = `{"name":"gideon-plus-two","cr":["508.1d","506.3"],"why":"Gideon Jura's +2 names Gideon as the defender",
 "setup":{"p0":{"battlefield":["Gideon Jura"]},"p1":{"battlefield":["Balduvian Bears"]}},
 "steps":[{"op":"activate","seat":0,"card":"p0:Gideon Jura","targets":["p1"],"ability_index":0},
  {"op":"pass_to","seat":0,"step":"upkeep","active":"p1"}]}`

// gideonAttackersDecision runs the +2 scenario to p1's upkeep, applies
// mutate (nil for none), then passes to p1's declare-attackers decision.
func gideonAttackersDecision(t *testing.T, mutate func(*Engine, state.ObjID)) (*Engine, *decision.Decision, state.ObjID, state.ObjID) {
	t.Helper()
	sc, err := decodeOracleScenario([]byte(gideonJuraPlusTwoToP1Upkeep))
	if err != nil {
		t.Fatal(err)
	}
	fails, transcript, run := runOracleScenario(testutil.CorpusRegistry(t), sc)
	if len(fails) != 0 {
		t.Fatalf("setup scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	e := run.e
	gideon, err := run.resolve("p0:Gideon Jura")
	if err != nil {
		t.Fatal(err)
	}
	bears, err := run.resolve("p1:Balduvian Bears")
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(e, gideon)
	}
	for i := 0; i < 50; i++ {
		d := e.Pending()
		if d == nil || e.G.Over {
			t.Fatal("game stopped before p1's attackers decision")
		}
		if d.Kind == decision.KAttackers {
			return e, d, gideon, bears
		}
		if err := run.answer(d, "to-attack"); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("no attackers decision")
	return nil, nil, 0, 0
}

func TestMustAttackEffectSourceNamesThePlaneswalker(t *testing.T) {
	e, d, gideon, bears := gideonAttackersDecision(t, nil)
	s := combat.AttackRequirements(asBoard(e), bears)
	if got := s.SatisfiedByOffer(0, gideon); got != 1 {
		t.Fatalf("an attack on Gideon satisfies %d requirements, want 1 (set %+v)", got, s)
	}
	if got := s.SatisfiedByOffer(0, 0); got != 0 {
		t.Fatalf("an attack on the player satisfies %d requirements, want 0", got)
	}
	n := 0
	for _, o := range d.Options {
		if o.Obj != bears {
			continue
		}
		n++
		if o.Battle != gideon || !o.Required {
			t.Fatalf("Bears option %+v: want only the Required attack on Gideon Jura", o)
		}
	}
	if n != 1 {
		t.Fatalf("Bears has %d attack options, want exactly the Gideon pair: %s", n, optionDump(d))
	}
}

// Once Gideon has left the battlefield the duty names nothing it could be
// discharged against: the Bears are released (not Required) and may attack
// the player.
func TestMustAttackEffectSourceReleasedWhenSourceLeaves(t *testing.T) {
	_, d, _, bears := gideonAttackersDecision(t, func(e *Engine, gideon state.ObjID) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: gideon, From: state.ZBattlefield, To: state.ZGraveyard})
	})
	player := false
	for _, o := range d.Options {
		if o.Obj != bears {
			continue
		}
		if o.Required {
			t.Fatalf("Bears still Required after Gideon left: %+v", o)
		}
		if o.Battle == 0 {
			player = true
		}
	}
	if !player {
		t.Fatalf("Bears offered no player attack after Gideon left: %s", optionDump(d))
	}
}

// End to end through the oracle harness: p1's next turn deals Gideon two
// damage (loyalty 8 -> 6), and the turn after carries no duty.
func TestGideonJuraPlusTwoOracleScenario(t *testing.T) {
	const sc = `{"name":"gideon-plus-two-full","cr":["508.1d"],"why":"the Bears attack Gideon Jura on p1's next turn only",
 "setup":{"p0":{"battlefield":["Gideon Jura"]},"p1":{"battlefield":["Balduvian Bears"]}},
 "steps":[{"op":"activate","seat":0,"card":"p0:Gideon Jura","targets":["p1"],"ability_index":0},
  {"op":"pass_to","seat":0,"step":"main2","active":"p0","expect":[{"card":"p0:Gideon Jura","counters":{"LOYALTY":8}}]},
  {"op":"pass_to","seat":0,"step":"main2","active":"p1","expect":[{"card":"p0:Gideon Jura","counters":{"LOYALTY":6}},{"life":{"p0":20}}]},
  {"op":"pass_to","seat":0,"step":"main2","active":"p0"},
  {"op":"pass_to","seat":0,"step":"main2","active":"p1","expect":[{"card":"p0:Gideon Jura","counters":{"LOYALTY":6}},{"life":{"p0":20}}]}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(sc))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}
