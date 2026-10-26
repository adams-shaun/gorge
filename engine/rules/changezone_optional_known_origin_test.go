package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 603.5 / 608.2d: a known-origin ChangeZone that carries Optional$ asks
// its chooser (Chooser$, else the controller) before each move; a decline
// drops that object only, and with no host "may" plays as "do" (R-9). The
// hand, hidden-pick and search walkers already posed the confirmation; the
// object path (Intimidation Campaign, Mistbreath Elder, Clash of Elements)
// never read the marker.

func runCZOptional(t *testing.T, sc string) OracleResult {
	t.Helper()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(sc))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	return res
}

func czOptionalLast(res OracleResult) OracleSnapshot {
	return res.Snapshots[len(res.Snapshots)-1]
}

func czOptionalHas(list []string, name string) bool {
	for _, s := range list {
		if s == name {
			return true
		}
	}
	return false
}

func intimidationScenario(answers string) string {
	return `{"name":"intimidation-campaign","cr":["603.5","608.2d"],"why":"the crime trigger's may-return is declinable",
 "setup":{"p0":{"battlefield":["Intimidation Campaign"],"hand":["Shock"]},"p1":{}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1"]},{"op":"resolve","seat":0` + answers + `}]}`
}

func TestChangeZoneOptionalKnownOriginDeclineKeeps(t *testing.T) {
	no := runCZOptional(t, intimidationScenario(`,"answers":[{"kind":"choose","pick":["No"],"seat":0}]`))
	fin := czOptionalLast(no)
	if _, ok := snapPerm(fin, "p0:Intimidation Campaign"); !ok {
		t.Fatalf("declined: Campaign left the battlefield; hand %v", fin.Players[0].Hand)
	}
	if czOptionalHas(fin.Players[0].Hand, "Intimidation Campaign") {
		t.Fatalf("declined: Campaign is also in hand")
	}
	// Precondition: the crime trigger really fired (Shock resolved on p1).
	if fin.Players[1].Life != 18 {
		t.Fatalf("p1 life = %d, want 18 (Shock resolved)", fin.Players[1].Life)
	}

	yes := runCZOptional(t, intimidationScenario(`,"answers":[{"kind":"choose","pick":["Yes"],"seat":0}]`))
	fin = czOptionalLast(yes)
	if _, ok := snapPerm(fin, "p0:Intimidation Campaign"); ok {
		t.Fatalf("accepted: Campaign still on the battlefield")
	}
	if !czOptionalHas(fin.Players[0].Hand, "Intimidation Campaign") {
		t.Fatalf("accepted: Campaign not in hand: %v", fin.Players[0].Hand)
	}
}

func TestChangeZoneOptionalKnownOriginNoHostMoves(t *testing.T) {
	res := runCZOptional(t, intimidationScenario(``))
	fin := czOptionalLast(res)
	if fin.Players[1].Life != 18 {
		t.Fatalf("p1 life = %d, want 18 (Shock resolved)", fin.Players[1].Life)
	}
	if !czOptionalHas(fin.Players[0].Hand, "Intimidation Campaign") {
		t.Fatalf("no answer: Campaign not in hand (R-9 plays may as do): %v", fin.Players[0].Hand)
	}
}

// The setup drive already walks turn 1's upkeep, so the Elder is cast in main
// phase 1 and the trigger under test is p0's turn-3 upkeep.
func mistbreathScenario(battlefield, answers string) string {
	return `{"name":"mistbreath","cr":["603.5","608.2d"],"why":"the otherwise-branch may-return is declinable",
 "setup":{"p0":{"hand":["Mistbreath Elder"],"battlefield":[` + battlefield + `]},"p1":{}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Mistbreath Elder","mana":"G"},{"op":"resolve","seat":0},{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},{"op":"pass_to","seat":0,"step":"upkeep","active":"p0"},{"op":"resolve","seat":0` + answers + `}]}`
}

func TestMistbreathElderOptionalSelfReturn(t *testing.T) {
	// Lone Elder, decline: it stays and gains no counter.
	fin := czOptionalLast(runCZOptional(t, mistbreathScenario(``,
		`,"answers":[{"kind":"choose","pick":["No"],"seat":0}]`)))
	elder, ok := snapPerm(fin, "p0:Mistbreath Elder")
	if !ok {
		t.Fatalf("declined: Elder left the battlefield; hand %v", fin.Players[0].Hand)
	}
	if len(elder.Counters) != 0 {
		t.Fatalf("declined: Elder counters = %v, want none", elder.Counters)
	}
	// Lone Elder, accept: it returns.
	fin = czOptionalLast(runCZOptional(t, mistbreathScenario(``,
		`,"answers":[{"kind":"choose","pick":["Yes"],"seat":0}]`)))
	if _, ok := snapPerm(fin, "p0:Mistbreath Elder"); ok || !czOptionalHas(fin.Players[0].Hand, "Mistbreath Elder") {
		t.Fatalf("accepted: Elder should be in hand; hand %v", fin.Players[0].Hand)
	}
	// With a Bears beside it the Bears return, the Elder grows, and the
	// optional self-return is never posed (its condition fails): the only
	// answer consumed is the Bears pick.
	fin = czOptionalLast(runCZOptional(t, mistbreathScenario(`"Grizzly Bears"`,
		`,"answers":[{"kind":"choose","pick":["p0:Grizzly Bears"]}]`)))
	elder, ok = snapPerm(fin, "p0:Mistbreath Elder")
	if !ok || elder.Counters["P1P1"] != 1 {
		t.Fatalf("with Bears: Elder = %+v, %v; want on battlefield with a +1/+1 counter", elder, ok)
	}
	if !czOptionalHas(fin.Players[0].Hand, "Grizzly Bears") {
		t.Fatalf("with Bears: Bears not in hand: %v", fin.Players[0].Hand)
	}
}

func clashScenario(answers string) string {
	return `{"name":"clash-of-elements","cr":["603.5","608.2d"],"why":"the targeted permanent's owner chooses whether it goes on top",
 "setup":{"p0":{"hand":["Clash of Elements"]},"p1":{"battlefield":["Grizzly Bears"],"library_top":["Forest"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Clash of Elements","mana":"CUR","targets":["p1:Grizzly Bears"]},{"op":"resolve","seat":0` + answers + `}]}`
}

func TestClashOfElementsOwnerChooses(t *testing.T) {
	// The ask is posed to seat 1 (the Bears' owner), not the caster.
	noRes := runCZOptional(t, clashScenario(`,"answers":[{"kind":"choose","pick":["No"],"seat":1}]`))
	base := noRes.Snapshots[0].Players[1].LibraryCount
	if _, ok := snapPerm(noRes.Snapshots[0], "p1:Grizzly Bears"); !ok {
		t.Fatalf("setup: Bears not on the battlefield")
	}
	no := czOptionalLast(noRes)
	if no.Players[1].Life != 20 {
		t.Fatalf("declined: p1 life = %d, want 20", no.Players[1].Life)
	}
	if _, ok := snapPerm(no, "p1:Grizzly Bears"); ok {
		t.Fatalf("declined: Bears still on the battlefield")
	}
	if no.Players[1].LibraryCount != base+1 || no.Players[1].LibraryTop[0] != "Forest" {
		t.Fatalf("declined: p1 library = %d top %v, want %d with Forest still on top (Bears on the bottom)",
			no.Players[1].LibraryCount, no.Players[1].LibraryTop, base+1)
	}

	yes := czOptionalLast(runCZOptional(t, clashScenario(`,"answers":[{"kind":"choose","pick":["Yes"],"seat":1}]`)))
	if yes.Players[1].Life != 18 {
		t.Fatalf("accepted: p1 life = %d, want 18", yes.Players[1].Life)
	}
	if yes.Players[1].LibraryCount != base+1 || yes.Players[1].LibraryTop[0] != "Grizzly Bears" {
		t.Fatalf("accepted: p1 library = %d top %v, want %d with Bears on top",
			yes.Players[1].LibraryCount, yes.Players[1].LibraryTop, base+1)
	}
}
