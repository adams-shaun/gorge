package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// offeredExpect is an `offered` expectation for the given seat, kind, card.
func offeredExpect(seat int, kind, card string, want bool) string {
	w := "true"
	if !want {
		w = "false"
	}
	return `{"offered":{"seat":` + string(rune('0'+seat)) + `,"kind":"` + kind + `","card":"` + card + `"},"want":` + w + `}`
}

func runPhasesScenario(t *testing.T, raw string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(raw))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) > 0 {
		t.Fatalf("%s\n  transcript:\n    %s", strings.Join(res.Fails, "\n"), strings.Join(res.Transcript, "\n    "))
	}
}

// TestCantBeCastPhasesWindow pins CR 506.1 / 604.1 for Yuriko, Blade of the
// Mighty: "During combat, players can't cast spells" binds only in the
// combat steps. Spells are castable in main 1 and main 2 and refused to both
// players in declare attackers.
func TestCantBeCastPhasesWindow(t *testing.T) {
	t.Parallel()
	runPhasesScenario(t, `{"name":"yuriko-cast-window","cr":["506.1","604.1"],"why":"inline",
 "setup":{"p0":{"battlefield":["Yuriko, Blade of the Mighty","Grizzly Bears","Forest","Forest","Forest"],"hand":["Runeclaw Bear","Giant Growth"]},
  "p1":{"battlefield":["Forest"],"hand":["Giant Growth"]}},
 "steps":[
  {"op":"mana","seat":0,"mana":"GG","expect":[`+offeredExpect(0, "cast", "p0:Runeclaw Bear", true)+`,`+offeredExpect(0, "cast", "p0:Giant Growth", true)+`]},
  {"op":"cast","seat":0,"card":"p0:Runeclaw Bear","mana":"GG"},{"op":"resolve"},
  {"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p1"},
  {"op":"mana","seat":0,"mana":"GG","expect":[`+offeredExpect(0, "cast", "p0:Giant Growth", false)+`]},
  {"op":"pass","seat":0},
  {"op":"mana","seat":1,"mana":"GG","expect":[`+offeredExpect(1, "cast", "p1:Giant Growth", false)+`]},
  {"op":"pass_to","seat":0,"step":"main2","active":"p0"},
  {"op":"mana","seat":0,"mana":"GG","expect":[`+offeredExpect(0, "cast", "p0:Giant Growth", true)+`]}
 ]}`)
}

// TestCantBeActivatedPhasesWindow: Yuriko's second static forbids non-mana
// activated abilities only during combat; a mana ability is never forbidden.
func TestCantBeActivatedPhasesWindow(t *testing.T) {
	t.Parallel()
	runPhasesScenario(t, `{"name":"yuriko-activate-window","cr":["506.1","604.1"],"why":"inline",
 "setup":{"p0":{"battlefield":["Yuriko, Blade of the Mighty","Prodigal Pyromancer","Grizzly Bears","Forest"]}},
 "steps":[
  {"op":"pass_to","seat":0,"decision":"priority","expect":[`+offeredExpect(0, "activate", "p0:Prodigal Pyromancer", true)+`,`+offeredExpect(0, "activate", "p0:Forest", true)+`]},
  {"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p1"},
  {"op":"pass_to","seat":0,"step":"declare-blockers","active":"p0"},
  {"op":"pass_to","seat":0,"decision":"priority","expect":[`+offeredExpect(0, "activate", "p0:Prodigal Pyromancer", false)+`,`+offeredExpect(0, "activate", "p0:Forest", true)+`]}
 ]}`)
}

// TestHandToHandInstantOnlyDuringCombat: Hand to Hand's CantBeCast is
// ValidCard$ Instant, so a creature spell is castable in main 1 (narrower
// than Yuriko), and an instant is refused during combat but not outside it.
func TestHandToHandInstantOnlyDuringCombat(t *testing.T) {
	t.Parallel()
	runPhasesScenario(t, `{"name":"hand-to-hand-window","cr":["506.1","604.1"],"why":"inline",
 "setup":{"p0":{"battlefield":["Hand to Hand","Grizzly Bears","Forest","Forest","Forest"],"hand":["Runeclaw Bear","Giant Growth"]}},
 "steps":[
  {"op":"mana","seat":0,"mana":"GG","expect":[`+offeredExpect(0, "cast", "p0:Runeclaw Bear", true)+`,`+offeredExpect(0, "cast", "p0:Giant Growth", true)+`]},
  {"op":"cast","seat":0,"card":"p0:Runeclaw Bear","mana":"GG"},{"op":"resolve"},
  {"op":"attack","seat":0,"attackers":["p0:Grizzly Bears"],"defender":"p1"},
  {"op":"mana","seat":0,"mana":"GG","expect":[`+offeredExpect(0, "cast", "p0:Giant Growth", false)+`]}
 ]}`)
}
