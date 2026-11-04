package rules

import (
	"strconv"
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

	// CR 707.5: the copied text's "enters with" replacement takes effect, so a
	// template with printed counters adds them to the rider's copy of the
	// origin's. Kalonian Hydra enters with four +1/+1 counters (one more is
	// set up on it: it holds five), Surge Node with six charge counters.
	t.Run("template printed counters add to the origin's", func(t *testing.T) {
		for _, c := range []struct {
			card, kind string
			printed    int
			extra      int
		}{{"Kalonian Hydra", "P1P1", 4, 1}, {"Surge Node", "CHARGE", 6, 2}} {
			raw := `{"name":"dominion-saboteur-printed","cr":["707.5","707.9b"],"why":"printed plus copied","setup":{` +
				`"p0":{"hand":["Dominion Saboteur"]},` +
				`"p1":{"battlefield":["` + c.card + `"],"counters":{"` + c.card + `":{"` + c.kind + `":` + strconv.Itoa(c.extra) + `}}}},` +
				`"steps":[{"op":"cast","seat":0,"card":"p0:Dominion Saboteur","mana":"CCCCU"},` +
				`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:` + c.card + `"]}]}],` +
				`"expect":[]}`
			run := runETBCloneScenario(t, raw)
			origin := findOn(t, run.e, state.ZBattlefield, 1, c.card)
			copyID := findOn(t, run.e, state.ZBattlefield, 0, c.card)
			have := int(run.e.G.Obj(origin).Counter(c.kind))
			if have < c.printed+c.extra {
				t.Fatalf("%s: precondition: origin holds %d %s, want at least %d", c.card, have, c.kind, c.printed+c.extra)
			}
			if got, want := int(run.e.G.Obj(copyID).Counter(c.kind)), c.printed+have; got != want {
				t.Fatalf("%s copy %s counters = %d, want %d (printed %d + origin's %d)", c.card, c.kind, got, want, c.printed, have)
			}
		}
	})

	// The same rule without the rider: a plain Clone copying Kalonian Hydra
	// enters with the Hydra's four +1/+1 counters, a 4/4, not a 0/0 that dies.
	t.Run("plain clone applies the template's enters-with counters", func(t *testing.T) {
		raw := `{"name":"clone-printed","cr":["707.5"],"why":"Clone of Kalonian Hydra","setup":{` +
			`"p0":{"hand":["Clone"]},` +
			`"p1":{"battlefield":["Kalonian Hydra"]}},` +
			`"steps":[{"op":"cast","seat":0,"card":"p0:Clone","mana":"CCCU"},` +
			`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Kalonian Hydra"]}]}],` +
			`"expect":[{"card":"p0:Kalonian Hydra","pt":"4/4","counters":{"P1P1":4}}]}`
		run := runETBCloneScenario(t, raw)
		copyID := findOn(t, run.e, state.ZBattlefield, 0, "Kalonian Hydra")
		if got := run.e.G.Obj(copyID).Counter("P1P1"); got != 4 {
			t.Fatalf("Clone of Kalonian Hydra holds %d +1/+1 counters, want 4", got)
		}
	})
}
