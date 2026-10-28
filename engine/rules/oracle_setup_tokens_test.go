package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func runSetupTokensScenario(t *testing.T, raw string) (fails, transcript []string, run *oracleRun) {
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

func setupTokensOf(run *oracleRun, p state.PlayerID) []*state.Object {
	var out []*state.Object
	for _, id := range run.e.G.Zone(state.ZBattlefield, p) {
		if o := run.e.G.Obj(id); o.IsToken {
			out = append(out, o)
		}
	}
	return out
}

func TestOracleSetupPlacesTokens(t *testing.T) {
	const raw = `{"name":"setup-tokens","setup":{"p0":{"battlefield":["Grizzly Bears"],
 "tokens":[{"script":"r_1_1_goblin"},{"script":"r_1_1_goblin","counters":{"P1P1":2}},{"script":"c_a_clue_draw"}]},
 "p1":{"battlefield":["Grizzly Bears"]}},"steps":[]}`
	fails, transcript, run := runSetupTokensScenario(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	toks := setupTokensOf(run, 0)
	if len(toks) != 3 {
		t.Fatalf("p0 controls %d tokens, want 3", len(toks))
	}
	for _, o := range toks {
		if o.SummonSick {
			t.Errorf("token %s is summoning sick", o.Face().Name)
		}
	}
	first, err := run.resolve("p0:token:Goblin Token#1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := run.resolve("p0:token:Goblin Token#2")
	if err != nil {
		t.Fatal(err)
	}
	if n := run.e.G.Obj(second).Counter("P1P1"); n != 2 {
		t.Errorf("Goblin #2 has %d P1P1 counters, want 2", n)
	}
	if n := run.e.G.Obj(first).Counter("P1P1"); n != 0 {
		t.Errorf("Goblin #1 has %d P1P1 counters, want 0", n)
	}
	if n := run.e.G.Players[0].TokensCreatedThisTurn; n != 0 {
		t.Errorf("TokensCreatedThisTurn = %d at the setup checkpoint, want 0", n)
	}
	if diff := diffGames(run.e.G, replayFromLog(t, run.cfg, run.e.L.Events)); diff != "" {
		t.Fatalf("log-only setup replay differs:\n%s", diff)
	}
}

func TestOracleSetupTokensFireNothing(t *testing.T) {
	const raw = `{"name":"setup-tokens-quiet","setup":{"p0":{"battlefield":["Mirkwood Bats","Doubling Season"],
 "tokens":[{"script":"c_a_treasure_sac"}]}},"steps":[]}`
	fails, transcript, run := runSetupTokensScenario(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	if n := len(setupTokensOf(run, 0)); n != 1 {
		t.Errorf("p0 controls %d tokens, want 1 (no Doubling Season copy)", n)
	}
	if life := run.e.G.Players[1].Life; life != 20 {
		t.Errorf("p1 life = %d, want 20 (no Mirkwood Bats trigger)", life)
	}
	if n := len(run.e.G.Stack); n != 0 {
		t.Errorf("stack holds %d objects, want 0", n)
	}
}

func TestOracleSetupTokenRejectsUnknownScript(t *testing.T) {
	const raw = `{"name":"setup-tokens-bad","setup":{"p0":{"tokens":[{"script":"no_such_token"}]}},"steps":[]}`
	fails, _, _ := runSetupTokensScenario(t, raw)
	if len(fails) == 0 || !strings.Contains(strings.Join(fails, "\n"), `token script "no_such_token" is not in the corpus`) {
		t.Fatalf("fails = %v, want the unknown-script harness error", fails)
	}
}
