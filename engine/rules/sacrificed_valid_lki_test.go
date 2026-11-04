package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// "If you sacrificed a Klingon this way" is read from the sacrificed object as
// it last existed on the battlefield (CR 608.2h). A Klingon token has ceased to
// exist by the time Gowron's ability resolves (CR 704.5d), and it still counts:
// Gowron's controller draws a card and loses 1 life. A non-Klingon sacrifice
// draws nothing and costs no life.
func TestGowronCountsASacrificedKlingonToken(t *testing.T) {
	for _, c := range []struct {
		name, setup, pick string
		life              int
	}{
		{"klingon-token", `"battlefield":["Gowron, Eager for Battle"],"tokens":[{"script":"r_2_1_klingon_warrior_haste"}]`, "Klingon Warrior Token", 19},
		{"other-creature", `"battlefield":["Gowron, Eager for Battle","Grizzly Bears"]`, "Grizzly Bears", 20},
	} {
		t.Run(c.name, func(t *testing.T) {
			scenario := `{"name":"gowron-` + c.name + `","cr":["608.2h"],"why":"inline","setup":{"p0":{` + c.setup + `,"library_top":["Island"]}},` +
				`"steps":[{"op":"activate","seat":0,"card":"p0:Gowron, Eager for Battle","answers":[{"kind":"choose","pick":["` + c.pick + `"]}]},{"op":"resolve"}],` +
				`"expect":[{"life":{"p0":` + itoa(c.life) + `}}]}`
			res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
			}
		})
	}
}

// An artifact a continuous effect made a creature is a creature as it was
// sacrificed, though the card in the graveyard is not one (CR 608.2h): Emrakul's
// Evangel makes a Horror for it beside the one for itself.
func TestSacrificedValidReadsTheTypesItHadOnTheBattlefield(t *testing.T) {
	scenario := `{"name":"evangel-animated-artifact","cr":["608.2h"],"why":"inline","setup":{"p0":{"battlefield":["Emrakul's Evangel","Mind Stone","Ensoul Artifact"],` +
		`"attached":[{"card":"Ensoul Artifact","to":"p0:Mind Stone"}]}},` +
		`"steps":[{"op":"activate","seat":0,"card":"p0:Emrakul's Evangel","answers":[{"kind":"choose","pick":["X = 1"]},{"kind":"choose","pick":["Mind Stone"]}]},{"op":"resolve"}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	horrors := 0
	last := res.Snapshots[len(res.Snapshots)-1]
	for _, p := range last.Permanents {
		if strings.Contains(p.Name, "Eldrazi Horror") {
			horrors++
		}
	}
	if horrors != 2 {
		t.Fatalf("%d Eldrazi Horrors, want 2 (the Evangel and the animated Mind Stone)\n%s", horrors, strings.Join(res.Transcript, "\n"))
	}
}
