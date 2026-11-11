// oracle_setup_commander_test.go — a Commander oracle scenario can restate
// the three pieces of commander state the runner used to refuse: a commander
// outside the command zone (CR 903.9), its tax count (CR 903.8), and the
// combat damage a seat has taken from it (CR 903.10a).
package rules

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runOracleCommander decodes raw and plays it in audit mode (no XMage
// fixture), returning the run even when the scenario fails so a test can
// inspect the engine and the recorded decisions.
func runOracleCommander(t *testing.T, raw string) (*oracleRun, []string) {
	t.Helper()
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	fails, _, run := runOracleScenario(testutil.CorpusRegistry(t), sc)
	return run, fails
}

// commanderOnBattlefieldScenario is Isamaru placed as p0's commander on the
// battlefield, with p1's copy still in the command zone.
const commanderOnBattlefieldScenario = `{"name":"commander-on-battlefield","format":"commander","cr":["903.9"],"why":"a commander cast and settled on the battlefield",
"setup":{"p0":{"battlefield":["Isamaru, Hound of Konda"],"commanders":["Isamaru, Hound of Konda"]},
         "p1":{"command":["Isamaru, Hound of Konda"]}},
"steps":[]}`

// TestOracleSetupCommanderOnBattlefield: genesis seats a battlefield-placed
// commander in Config.Commanders and setup moves it out of the command zone
// to the listed zone, so the setup snapshot shows it on the battlefield with
// p0's command zone empty.
func TestOracleSetupCommanderOnBattlefield(t *testing.T) {
	run, fails := runOracleCommander(t, commanderOnBattlefieldScenario)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	ref, ok := run.refs["p0:Isamaru, Hound of Konda"]
	if !ok {
		t.Fatal("precondition: Isamaru's battlefield ref did not bind")
	}
	o := run.e.G.Obj(ref)
	if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("Isamaru zone/controller = %v/%d, want battlefield/p0", o.Zone, o.Controller)
	}
	if got := run.e.G.Players[0].Commanders; len(got) != 1 || got[0] != ref {
		t.Fatalf("p0 Commanders = %v, want [%d] (the battlefield object)", got, ref)
	}
	if len(run.e.G.Zone(state.ZCommand, 0)) != 0 {
		t.Fatalf("p0 command zone = %v, want empty", run.e.G.Zone(state.ZCommand, 0))
	}
	setup := run.snaps[0]
	if len(setup.Players[0].Command) != 0 {
		t.Fatalf("setup snapshot p0 command = %v, want empty", setup.Players[0].Command)
	}
	found := false
	for _, p := range setup.Permanents {
		if p.Ref == "p0:Isamaru, Hound of Konda" && p.Controller == 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("setup snapshot has no p0 Isamaru permanent: %+v", setup.Permanents)
	}
}

// commanderTaxSetup is p0's Isamaru in the command zone with a restated cast
// count of 1: its next cast costs {2}{W}.
const commanderTaxSetup = `{"name":"commander-tax","format":"commander","cr":["903.8"],"why":"a commander cast once before the cut",
"setup":{"p0":{"command":["Isamaru, Hound of Konda"],"commander_casts":{"Isamaru, Hound of Konda":1}},
         "p1":{}}`

// TestOracleSetupCommanderCastsSetTax: the seeded count raises the tax, so a
// cast funded by {W} alone is not offered and one funded by {W}{C}{C} is.
func TestOracleSetupCommanderCastsSetTax(t *testing.T) {
	unpaid := commanderTaxSetup + `,"steps":[{"op":"cast","seat":0,"card":"p0:Isamaru, Hound of Konda","mana":"W"}]}`
	run, fails := runOracleCommander(t, unpaid)
	if len(fails) == 0 {
		t.Fatalf("a {W} cast paid the {2} tax: %s", strings.Join(run.log, "\n"))
	}
	if !strings.Contains(strings.Join(fails, "\n"), "Isamaru") {
		t.Fatalf("cast refusal does not name the commander: %v", fails)
	}

	paid := commanderTaxSetup + `,"steps":[{"op":"cast","seat":0,"card":"p0:Isamaru, Hound of Konda","mana":"WCC"}]}`
	run, fails = runOracleCommander(t, paid)
	if len(fails) != 0 {
		t.Fatalf("a {W}{C}{C} cast was refused: %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	last := run.snaps[len(run.snaps)-1]
	if len(last.Stack) != 1 || last.Stack[0].Source != "p0:Isamaru, Hound of Konda" {
		t.Fatalf("stack = %+v, want Isamaru on the stack", last.Stack)
	}
}

// commanderDamageSetup starts p1 at 40 life with 20 commander damage already
// taken from p0's Isamaru; one unblocked Isamaru attack makes it 22.
const commanderDamageSetup = `{"name":"commander-damage","format":"commander","cr":["903.10a","704.6c"],"why":"20 commander damage already taken, then one more hit",
"setup":{"p0":{"battlefield":["Isamaru, Hound of Konda"],"commanders":["Isamaru, Hound of Konda"]},
         "p1":{"life":40,"commander_damage":{"p0:Isamaru, Hound of Konda":20}}}`

// TestOracleSetupCommanderDamageTally: the setup event folds into p1's
// CmdDamage without changing life, and the next two combat damage from the
// same commander reaches CR 704.6c's 21 and p1 loses.
func TestOracleSetupCommanderDamageTally(t *testing.T) {
	setupRun, fails := runOracleCommander(t, commanderDamageSetup+`,"steps":[]}`)
	if len(fails) != 0 {
		t.Fatalf("setup scenario failed: %v\n%s", fails, strings.Join(setupRun.log, "\n"))
	}
	if got := setupRun.e.G.Players[1].CmdDamage; len(got) != 1 || got[0] != 20 {
		t.Fatalf("p1 CmdDamage = %v, want [20]", got)
	}
	if got := setupRun.e.G.Players[1].Life; got != 40 {
		t.Fatalf("p1 life after setup = %d, want 40 (commander damage is not life loss)", got)
	}

	raw := commanderDamageSetup + `,"steps":[
		{"op":"attack","seat":0,"attackers":["p0:Isamaru, Hound of Konda"],"defender":"p1"},
		{"op":"pass","seat":0},
		{"op":"pass","seat":1},
		{"op":"pass","seat":0},
		{"op":"pass","seat":1}]}`
	run, fails := runOracleCommander(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	last := run.snaps[len(run.snaps)-1]
	if !last.Over || last.Winner == nil || *last.Winner != 0 {
		t.Fatalf("after 22 commander damage: over=%v winner=%v, want p0's win", last.Over, last.Winner)
	}
	if got := run.e.G.Players[1].CmdDamage; len(got) != 1 || got[0] != 22 {
		t.Fatalf("p1 CmdDamage after the attack = %v, want [22]", got)
	}
}

// commanderGraveyardScenario is a commander the cut shows in the graveyard:
// it must stay there, with no CR 903.9a command-zone offer during setup.
const commanderGraveyardScenario = `{"name":"commander-graveyard","format":"commander","cr":["903.9a"],"why":"a commander that died and stayed in the graveyard",
"setup":{"p0":{"graveyard":["Isamaru, Hound of Konda"],"commanders":["Isamaru, Hound of Konda"]},"p1":{}},
"steps":[]}`

// TestOracleSetupCommanderInGraveyardAsksNothing: the setup move into the
// graveyard is not a fresh CR 903.9a arrival, so the first state-based check
// asks nothing and the setup snapshot still has Isamaru in the graveyard.
func TestOracleSetupCommanderInGraveyardAsksNothing(t *testing.T) {
	run, fails := runOracleCommander(t, commanderGraveyardScenario)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	for _, d := range run.decisions {
		if d.Step == -1 {
			t.Fatalf("setup posed a decision at step -1: %+v", d)
		}
	}
	ref, ok := run.refs["p0:Isamaru, Hound of Konda"]
	if !ok {
		t.Fatal("precondition: Isamaru's graveyard ref did not bind")
	}
	if o := run.e.G.Obj(ref); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Isamaru zone = %v, want graveyard", o.Zone)
	}
	if gy := run.snaps[0].Players[0].Graveyard; len(gy) != 1 || gy[0] != "Isamaru, Hound of Konda" {
		t.Fatalf("setup snapshot p0 graveyard = %v, want Isamaru", gy)
	}
}

// TestOracleSetupCommanderFieldsRefused: each field's harness errors name the
// seat and the field, never degrade to a silent no-op.
func TestOracleSetupCommanderFieldsRefused(t *testing.T) {
	const commanderSetup = `"format":"commander","setup":`
	cases := []struct {
		name string
		raw  string
		seat string
		key  string
	}{
		{
			name: "constructed commanders",
			raw: `{"name":"x","setup":{"p0":{"battlefield":["Isamaru, Hound of Konda"],` +
				`"commanders":["Isamaru, Hound of Konda"]},"p1":{}},"steps":[]}`,
			seat: "p0", key: "commanders",
		},
		{
			name: "constructed commander_casts",
			raw: `{"name":"x","setup":{"p0":{"command":["Isamaru, Hound of Konda"],` +
				`"commander_casts":{"Isamaru, Hound of Konda":1}},"p1":{}},"steps":[]}`,
			seat: "p0", key: "commander_casts",
		},
		{
			name: "constructed commander_damage",
			raw: `{"name":"x","setup":{"p0":{"battlefield":["Isamaru, Hound of Konda"]},` +
				`"p1":{"commander_damage":{"p0:Isamaru, Hound of Konda":3}}},"steps":[]}`,
			seat: "p1", key: "commander_damage",
		},
		{
			name: "name not placed",
			raw: `{"name":"x",` + commanderSetup + `{"p0":{"battlefield":["Isamaru, Hound of Konda"],` +
				`"commanders":["Isamaru, Hound of Konda","Grizzly Bears"]},"p1":{}},"steps":[]}`,
			seat: "p0", key: "commanders",
		},
		{
			name: "name placed twice",
			raw: `{"name":"x",` + commanderSetup + `{"p0":{"battlefield":["Isamaru, Hound of Konda","Isamaru, Hound of Konda"],` +
				`"commanders":["Isamaru, Hound of Konda"]},"p1":{}},"steps":[]}`,
			seat: "p0", key: "placed more than once",
		},
		{
			name: "three commanders",
			raw: `{"name":"x",` + commanderSetup + `{"p0":{"battlefield":["Isamaru, Hound of Konda","Grizzly Bears","Silvercoat Lion"],` +
				`"commanders":["Isamaru, Hound of Konda","Grizzly Bears","Silvercoat Lion"]},"p1":{}},"steps":[]}`,
			seat: "p0", key: "at most 2",
		},
		{
			name: "cast count zero",
			raw: `{"name":"x",` + commanderSetup + `{"p0":{"command":["Isamaru, Hound of Konda"],` +
				`"commander_casts":{"Isamaru, Hound of Konda":0}},"p1":{}},"steps":[]}`,
			seat: "p0", key: "commander_casts",
		},
		{
			name: "damage above twenty",
			raw: `{"name":"x",` + commanderSetup + `{"p0":{"battlefield":["Isamaru, Hound of Konda"],` +
				`"commanders":["Isamaru, Hound of Konda"]},` +
				`"p1":{"commander_damage":{"p0:Isamaru, Hound of Konda":21}}},"steps":[]}`,
			seat: "p1", key: "commander_damage",
		},
		{
			name: "damage key not a commander",
			raw: `{"name":"x",` + commanderSetup + `{"p0":{"battlefield":["Isamaru, Hound of Konda","Grizzly Bears"],` +
				`"commanders":["Isamaru, Hound of Konda"]},` +
				`"p1":{"commander_damage":{"p0:Grizzly Bears":5}}},"steps":[]}`,
			seat: "p1", key: "not a commander",
		},
	}
	reg := testutil.CorpusRegistry(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := RunOracleScenarioJSON(reg, []byte(c.raw))
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			got := strings.Join(res.Fails, "\n")
			if !strings.Contains(got, "harness:") || !strings.Contains(got, c.seat) || !strings.Contains(got, c.key) {
				t.Fatalf("fails = %q, want a harness error naming %q and %q", got, c.seat, c.key)
			}
		})
	}
}

// TestLogReplayGameSeedsCommanderCasts: the Config seed is genesis state, not
// logged state, so logReplayGame must read it back from Config exactly as New
// did. The unpaid-tax run cast nothing, so the seed is still 1.
func TestLogReplayGameSeedsCommanderCasts(t *testing.T) {
	run, _ := runOracleCommander(t, commanderTaxSetup+`,"steps":[
		{"op":"cast","seat":0,"card":"p0:Isamaru, Hound of Konda","mana":"W"}]}`)
	if run.cfg.CommanderCasts == nil || len(run.cfg.CommanderCasts[0]) != 1 || run.cfg.CommanderCasts[0][0] != 1 {
		t.Fatalf("cfg.CommanderCasts = %v, want [[1]]", run.cfg.CommanderCasts)
	}
	if got := run.e.G.Players[0].CmdCasts[0]; got != 1 {
		t.Fatalf("live CmdCasts[0] = %d, want 1", got)
	}
	rebuilt := logReplayGame(run.cfg, run.e.L.Events)
	if diff := gameStateDiff(run.e.G, rebuilt); diff != "" {
		t.Fatalf("log replay differs:\n%s", diff)
	}
	if got := rebuilt.Players[0].CmdCasts[0]; got != 1 {
		t.Fatalf("replayed CmdCasts[0] = %d, want 1", got)
	}
}

// TestOracleConstructedScenarioUnchanged: a constructed scenario that uses
// none of the fields produces the same transcript, event log and snapshots as
// one whose JSON carries the fields empty, and its Config keeps the new field
// nil so every other Config is byte-identical.
func TestOracleConstructedScenarioUnchanged(t *testing.T) {
	const base = `{"name":"constructed-unchanged","setup":{"p0":{"hand":["Shock"]},"p1":{"life":2}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1"]},{"op":"resolve"}]}`
	const withEmptyFields = `{"name":"constructed-unchanged","setup":{` +
		`"p0":{"hand":["Shock"],"commanders":[],"commander_casts":{},"commander_damage":{}},` +
		`"p1":{"life":2}},"steps":[{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1"]},{"op":"resolve"}]}`

	absentRun, absentFails := runOracleCommander(t, base)
	emptyRun, emptyFails := runOracleCommander(t, withEmptyFields)
	if len(absentFails) != 0 || len(emptyFails) != 0 {
		t.Fatalf("scenario failed: absent=%v empty=%v", absentFails, emptyFails)
	}
	if absentRun.cfg.CommanderCasts != nil {
		t.Fatalf("cfg.CommanderCasts = %v, want nil", absentRun.cfg.CommanderCasts)
	}
	if emptyRun.cfg.CommanderCasts != nil {
		t.Fatalf("empty-field cfg.CommanderCasts = %v, want nil", emptyRun.cfg.CommanderCasts)
	}
	if !reflect.DeepEqual(absentRun.e.L.Events, emptyRun.e.L.Events) {
		t.Fatalf("event log changed with empty fields:\nabsent=%v\nempty=%v", absentRun.e.L.Events, emptyRun.e.L.Events)
	}
	if !reflect.DeepEqual(absentRun.snaps, emptyRun.snaps) {
		t.Fatalf("snapshots changed with empty fields")
	}
	if !reflect.DeepEqual(absentRun.log, emptyRun.log) {
		t.Fatalf("transcript changed with empty fields:\nabsent=%s\nempty=%s",
			strings.Join(absentRun.log, "\n"), strings.Join(emptyRun.log, "\n"))
	}
}
