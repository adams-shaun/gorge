package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func runSetupAttachedScenario(t *testing.T, raw string) (fails, transcript []string, run *oracleRun) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, transcript, run = runOracleScenario(reg, sc)
	return fails, transcript, run
}

func TestOracleSetupAttachedAuraNamesItsBearer(t *testing.T) {
	const raw = `{"name":"attached-bearer","setup":{"p0":{"battlefield":["Pacifism"],
 "attached":[{"card":"Pacifism","to":"p1:Grizzly Bears#2"}]},
 "p1":{"battlefield":["Grizzly Bears","Grizzly Bears"]}},"steps":[]}`
	fails, transcript, run := runSetupAttachedScenario(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	aura := run.e.G.Obj(run.refs["p0:Pacifism"])
	bearer, first := run.refs["p1:Grizzly Bears#2"], run.refs["p1:Grizzly Bears"]
	if bearer == 0 || bearer == first {
		t.Fatalf("bearer refs not distinct: #2=%d #1=%d", bearer, first)
	}
	if aura.Zone != state.ZBattlefield {
		t.Fatalf("Pacifism is in zone %v, want the battlefield", aura.Zone)
	}
	if aura.AttachedTo != bearer {
		t.Errorf("Pacifism attached to %d, want Grizzly Bears#2 (%d); #1 is %d", aura.AttachedTo, bearer, first)
	}
	for _, line := range transcript {
		if strings.Contains(line, "choose") && strings.Contains(line, "p0") {
			t.Errorf("setup posed a choice to p0: %s", line)
		}
	}
	for _, ev := range run.e.L.Events {
		if ev.Kind == events.Attach && ev.Obj == aura.ID && len(ev.IDs) == 1 && ev.IDs[0] != bearer {
			t.Errorf("Pacifism was attached to %d during setup, want only %d", ev.IDs[0], bearer)
		}
	}
	if diff := diffGames(run.e.G, replayFromLog(t, run.cfg, run.e.L.Events)); diff != "" {
		t.Fatalf("log-only setup replay differs:\n%s", diff)
	}
}

func TestOracleSetupAttachedAuraFiresNothing(t *testing.T) {
	const raw = `{"name":"attached-quiet","setup":{"p0":{"battlefield":["Bramble Elemental","Rancor"],
 "attached":[{"card":"Rancor","to":"p0:Bramble Elemental"}]}},"steps":[]}`
	fails, transcript, run := runSetupAttachedScenario(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	bramble := run.refs["p0:Bramble Elemental"]
	if got := run.e.G.Obj(run.refs["p0:Rancor"]).AttachedTo; got != bramble {
		t.Fatalf("Rancor attached to %d, want Bramble Elemental (%d)", got, bramble)
	}
	if n := len(setupTokensOf(run, 0)); n != 0 {
		t.Errorf("p0 controls %d tokens, want 0 (no Saproling from the attach trigger)", n)
	}
	if n := len(run.e.G.Stack); n != 0 {
		t.Errorf("stack holds %d objects, want 0", n)
	}
	if p := run.e.Derived(bramble).Power; p != 6 {
		t.Errorf("Bramble Elemental power = %d, want 6", p)
	}
}

func TestOracleSetupAttachedAuraKeepsRefOrdinals(t *testing.T) {
	const raw = `{"name":"attached-ordinals","setup":{"p0":{"battlefield":["Pacifism","Pacifism"],
 "attached":[{"card":"Pacifism#2","to":"p1:Grizzly Bears"}]},
 "p1":{"battlefield":["Grizzly Bears"]}},"steps":[]}`
	fails, transcript, run := runSetupAttachedScenario(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	bear := run.refs["p1:Grizzly Bears"]
	if got := run.e.G.Obj(run.refs["p0:Pacifism#2"]).AttachedTo; got != bear {
		t.Errorf("p0:Pacifism#2 attached to %d, want %d", got, bear)
	}
	if run.refs["p0:Pacifism"] == run.refs["p0:Pacifism#2"] {
		t.Error("the two Pacifism refs bind one object")
	}
}

func TestOracleSetupAttachedRejects(t *testing.T) {
	for name, raw := range map[string]string{
		"card not on battlefield": `{"name":"a","setup":{"p0":{"battlefield":["Grizzly Bears"],
 "attached":[{"card":"Pacifism","to":"p0:Grizzly Bears"}]}},"steps":[]}`,
		"bearer in hand": `{"name":"b","setup":{"p0":{"battlefield":["Pacifism"],"hand":["Grizzly Bears"],
 "attached":[{"card":"Pacifism","to":"p0:Grizzly Bears"}]}},"steps":[]}`,
		"malformed to": `{"name":"c","setup":{"p0":{"battlefield":["Pacifism"],
 "attached":[{"card":"Pacifism","to":"Grizzly Bears"}]}},"steps":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			fails, _, _ := runSetupAttachedScenario(t, raw)
			if len(fails) == 0 {
				t.Fatal("scenario passed, want a harness error")
			}
		})
	}
}
