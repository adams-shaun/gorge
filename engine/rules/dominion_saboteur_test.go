package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Dominion Saboteur's as-enters copy: "You may have this creature enter as a
// copy of any artifact or creature on the battlefield, except it isn't
// legendary. If you do, it enters with additional counters on it equal to the
// same number and kinds of counters the copied permanent has on it." The
// Clone body remembers its copy origin and chains a PutCounter Defined$ Self
// | ETB$ True | CounterType$ EachFromSource | EachFromSource$ Remembered, so
// the copy gets the origin's counters while it is entering (CR 707.9b,
// 614.12): its own enters trigger and the state-based actions see them.
func TestDominionSaboteurGorge(t *testing.T) {
	t.Parallel()

	t.Run("creature counters seen by its enters trigger", func(t *testing.T) {
		raw := `{"name":"dominion-saboteur-creature","cr":["707.9b","614.12"],"why":"counter copy","setup":{` +
			`"p0":{"hand":["Dominion Saboteur"]},` +
			`"p1":{"battlefield":["Kitchen Finks"],"counters":{"Kitchen Finks":{"P1P1":2}}}},` +
			`"steps":[{"op":"cast","seat":0,"card":"p0:Dominion Saboteur","mana":"CCCCU"},` +
			`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Kitchen Finks"]}]}],` +
			`"expect":[{"card":"p0:Kitchen Finks","pt":"5/4","counters":{"P1P1":2}},` +
			`{"card":"p1:Kitchen Finks","pt":"5/4","counters":{"P1P1":2}}]}`
		run := runETBCloneScenario(t, raw)
		copyID := findOn(t, run.e, state.ZBattlefield, 0, "Kitchen Finks")
		if got := run.e.G.Obj(copyID).Counter("P1P1"); got != 2 {
			t.Fatalf("copy P1P1 counters = %d, want 2", got)
		}
		counterAt, triggerAt := -1, -1
		for i := range run.e.L.Events {
			ev := &run.e.L.Events[i]
			if ev.Kind == events.CounterChange && ev.Obj == copyID && ev.Counter == "P1P1" {
				counterAt = i
			}
			if ev.Kind == events.TriggerPush && ev.Obj == copyID && triggerAt < 0 {
				triggerAt = i
			}
		}
		if counterAt < 0 {
			t.Fatal("no P1P1 CounterChange on the copy")
		}
		if triggerAt < 0 {
			t.Fatal("precondition: the copy's enters trigger was never pushed")
		}
		if counterAt > triggerAt {
			t.Fatalf("copy's counters (event %d) landed after its enters trigger (event %d)", counterAt, triggerAt)
		}
	})

	t.Run("artifact charge counters", func(t *testing.T) {
		raw := `{"name":"dominion-saboteur-artifact","cr":["707.9b"],"why":"counter copy","setup":{` +
			`"p0":{"hand":["Dominion Saboteur"]},` +
			`"p1":{"battlefield":["Ornithopter"],"counters":{"Ornithopter":{"CHARGE":2}}}},` +
			`"steps":[{"op":"cast","seat":0,"card":"p0:Dominion Saboteur","mana":"CCCCU"},` +
			`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Ornithopter"]}]}],` +
			`"expect":[{"card":"p0:Ornithopter","pt":"0/2","counters":{"CHARGE":2}}]}`
		run := runETBCloneScenario(t, raw)
		copyID := findOn(t, run.e, state.ZBattlefield, 0, "Ornithopter")
		if got := run.e.G.Obj(copyID).Counter("CHARGE"); got != 2 {
			t.Fatalf("copy CHARGE counters = %d, want 2", got)
		}
		if got := run.e.G.Obj(copyID).Counter("P1P1"); got != 0 {
			t.Fatalf("copy P1P1 counters = %d, want 0", got)
		}
	})

	t.Run("legendary template copies nonlegendary", func(t *testing.T) {
		raw := `{"name":"dominion-saboteur-legendary","cr":["707.9b"],"why":"NonLegendary$ exception","setup":{` +
			`"p0":{"hand":["Dominion Saboteur"]},` +
			`"p1":{"battlefield":["Isamaru, Hound of Konda"],"counters":{"Isamaru, Hound of Konda":{"P1P1":1}}}},` +
			`"steps":[{"op":"cast","seat":0,"card":"p0:Dominion Saboteur","mana":"CCCCU"},` +
			`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Isamaru, Hound of Konda"]}]}],` +
			`"expect":[{"card":"p0:Isamaru, Hound of Konda","pt":"3/3","counters":{"P1P1":1},"no_types":["Legendary"]},` +
			`{"card":"p1:Isamaru, Hound of Konda","types":["Legendary"]}]}`
		run := runETBCloneScenario(t, raw)
		copyID := findOn(t, run.e, state.ZBattlefield, 0, "Isamaru, Hound of Konda")
		if got := run.e.G.Obj(copyID).Counter("P1P1"); got != 1 {
			t.Fatalf("copy P1P1 counters = %d, want 1", got)
		}
	})

	t.Run("template without counters adds none", func(t *testing.T) {
		raw := `{"name":"dominion-saboteur-no-counters","cr":["707.9b"],"why":"nothing to copy","setup":{` +
			`"p0":{"hand":["Dominion Saboteur"]},` +
			`"p1":{"battlefield":["Ornithopter"]}},` +
			`"steps":[{"op":"cast","seat":0,"card":"p0:Dominion Saboteur","mana":"CCCCU"},` +
			`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Ornithopter"]}]}],` +
			`"expect":[{"card":"p0:Ornithopter","pt":"0/2"}]}`
		run := runETBCloneScenario(t, raw)
		copyID := findOn(t, run.e, state.ZBattlefield, 0, "Ornithopter")
		o := run.e.G.Obj(copyID)
		for _, k := range []string{"P1P1", "CHARGE"} {
			if got := o.Counter(k); got != 0 {
				t.Fatalf("copy gained %d %s counters from a counterless template", got, k)
			}
		}
	})

	t.Run("decline enters as itself with no counters", func(t *testing.T) {
		raw := `{"name":"dominion-saboteur-decline","cr":["614.1c","707.9b"],"why":"optional decline","setup":{` +
			`"p0":{"hand":["Dominion Saboteur"]},` +
			`"p1":{"battlefield":["Kitchen Finks"],"counters":{"Kitchen Finks":{"P1P1":2}}}},` +
			`"steps":[{"op":"cast","seat":0,"card":"p0:Dominion Saboteur","mana":"CCCCU"},` +
			`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["Enter as itself"]}]}],` +
			`"expect":[{"card":"p0:Dominion Saboteur","pt":"3/3","counters":{}}]}`
		run := runETBCloneScenario(t, raw)
		id := findOn(t, run.e, state.ZBattlefield, 0, "Dominion Saboteur")
		o := run.e.G.Obj(id)
		for _, k := range []string{"P1P1", "CHARGE"} {
			if got := o.Counter(k); got != 0 {
				t.Fatalf("declined entry gained %d %s counters", got, k)
			}
		}
	})
}
