package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// cloneTemplateEntryScenario has p0 cast Clever Impersonator copying p1's
// Cindering Cutthroat ("enters with a +1/+1 counter if an opponent lost life
// this turn"). When ping is set, p0's Prodigal Pyromancer first costs p1 a
// life, so the template's condition is true for the copy's controller.
func cloneTemplateEntryScenario(ping bool) string {
	pre := ""
	if ping {
		pre = `{"op":"activate","seat":0,"card":"p0:Prodigal Pyromancer","targets":["p1"]},{"op":"resolve","seat":0},`
	}
	return `{"name":"clone-template-entry-condition","cr":["707.2","614.1c"],"why":"the copy's entry counter gate reads the template's SVars","setup":{` +
		`"p0":{"hand":["Clever Impersonator"],"battlefield":["Prodigal Pyromancer"]},"p1":{"battlefield":["Cindering Cutthroat"]}},` +
		`"steps":[` + pre + `{"op":"cast","seat":0,"card":"p0:Clever Impersonator","mana":"CCUU"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Cindering Cutthroat"]}]}],"expect":[]}`
}

// TestCloneTemplateEntryConditionReadsTemplateSVars: a clone's copied
// "enters with a +1/+1 counter if an opponent lost life this turn" is gated by
// the TEMPLATE's CheckSVar$ table (CR 707.2), not the entrant's own face, which
// has no such SVar and would let the gate pass unconditionally.
func TestCloneTemplateEntryConditionReadsTemplateSVars(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		ping bool
		want int32
	}{
		{"no opponent lost life", false, 0},
		{"an opponent lost life", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := runETBCloneScenario(t, cloneTemplateEntryScenario(tc.ping)).e
			lost := e.G.Players[1].LifeLostThisTurn
			if (lost > 0) != tc.ping {
				t.Fatalf("precondition: opponent LifeLostThisTurn = %d, ping = %v", lost, tc.ping)
			}
			cp := findOn(t, e, state.ZBattlefield, 0, "Cindering Cutthroat")
			if got := e.G.Obj(cp).Counter("P1P1"); got != tc.want {
				t.Fatalf("copy P1P1 counters = %d, want %d", got, tc.want)
			}
		})
	}
}
