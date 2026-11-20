package rules

// CR 310.10 (a Battle that enters other than as a cast Battle spell has its
// controller choose an opponent as its protector) over CR 707.2 and 614.12:
// a permanent that enters as a copy of a Siege enters as a Battle, so the
// protector choice is posed for it though ClonePermanent installs the Battle
// face only after the entry move.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func protectorChoices(e *Engine, id state.ObjID) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Counter == "protector" && ev.Obj == id {
			out = append(out, ev)
		}
	}
	return out
}

func TestCopiedBattleRecordsProtector(t *testing.T) {
	t.Parallel()
	const siege = "Invasion of Shandalar"
	scenario := func(players, answers string) string {
		return `{"name":"copied-battle-protector",` + players + `"cr":["310.10","707.2"],"why":"copied battle protector","setup":{` +
			`"p0":{"hand":["Clever Impersonator"]},"p1":{"battlefield":["` + siege + `"]}},` +
			`"steps":[{"op":"cast","seat":0,"card":"p0:Clever Impersonator","mana":"CCUU"},` +
			`{"op":"resolve","answers":[{"kind":"choose","pick":["p1:` + siege + `"]}` + answers + `]}],"expect":[]}`
	}
	t.Run("two-seats-auto-recorded", func(t *testing.T) {
		e := runETBCloneScenario(t, scenario("", "")).e
		cp := findOn(t, e, state.ZBattlefield, 0, siege)
		if got := e.G.Obj(cp).Counter("DEFENSE"); got != 4 {
			t.Fatalf("precondition: the copy has %d defense counters, want 4", got)
		}
		got := protectorChoices(e, cp)
		if len(got) != 1 || got[0].Player != 1 {
			t.Fatalf("copy's protector choices = %+v, want one naming seat 1", got)
		}
		if o := e.G.Obj(cp); !o.ProtectorValid || o.Protector != 1 {
			t.Fatalf("copy's protector = %v/%v, want seat 1", o.ProtectorValid, o.Protector)
		}
	})
	t.Run("three-seats-asked", func(t *testing.T) {
		reg := testutil.CorpusRegistry(t)
		sc, err := decodeOracleScenario([]byte(scenario(`"players":3,`, `,{"kind":"choose","pick":["p2"]}`)))
		if err != nil {
			t.Fatal(err)
		}
		fails, tr, run := runOracleScenario(reg, sc)
		if len(fails) != 0 {
			t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(tr, "\n"))
		}
		cp := findOn(t, run.e, state.ZBattlefield, 0, siege)
		got := protectorChoices(run.e, cp)
		if len(got) != 1 || got[0].Player != 2 {
			t.Fatalf("copy's protector choices = %+v, want one naming seat 2", got)
		}
	})
}
