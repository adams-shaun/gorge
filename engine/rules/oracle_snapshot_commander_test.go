// oracle_snapshot_commander_test.go — a Commander game's snapshots carry each
// seat's commander tax counts (CR 903.8) and the commander damage it has taken
// (CR 903.10a); a constructed game's snapshots carry neither key.
package rules

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestOracleSnapshotCommanderCasts: a restated cast count is visible on the
// setup snapshot, and one more cast from the command zone increments it.
func TestOracleSnapshotCommanderCasts(t *testing.T) {
	const name = "Isamaru, Hound of Konda"
	raw := `{"name":"snapshot-commander-casts","format":"commander","cr":["903.8"],"why":"a commander cast once before the cut, then again",
"setup":{"p0":{"command":["` + name + `"],"commander_casts":{"` + name + `":1}},
         "p1":{}},
"steps":[]}`
	run, fails := runOracleCommander(t, raw)
	if len(fails) != 0 {
		t.Fatalf("setup scenario failed: %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	if len(run.snaps) == 0 {
		t.Fatal("precondition: the setup snapshot was not recorded")
	}
	if got, want := run.snaps[0].Players[0].CommanderCasts, map[string]int32{name: 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("setup snapshot p0 CommanderCasts = %v, want %v", got, want)
	}
	if got := run.snaps[0].Players[1].CommanderCasts; got != nil {
		t.Fatalf("p1 CommanderCasts = %v, want nil", got)
	}

	cast := strings.TrimSuffix(raw, `}`) + `,
"steps":[{"op":"cast","seat":0,"card":"p0:` + name + `","mana":"WCC"}]}`
	run, fails = runOracleCommander(t, cast)
	if len(fails) != 0 {
		t.Fatalf("cast scenario failed: %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	last := run.snaps[len(run.snaps)-1]
	if got, want := last.Players[0].CommanderCasts, map[string]int32{name: 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after the second cast p0 CommanderCasts = %v, want %v", got, want)
	}
	if got := last.Players[0].CommanderDamage; got != nil {
		t.Fatalf("p0 CommanderDamage = %v, want nil", got)
	}
}

// TestOracleSnapshotCommanderDamage: an unblocked commander attack is 2 combat
// damage from p0's commander to p1, visible on the post-combat checkpoint.
func TestOracleSnapshotCommanderDamage(t *testing.T) {
	const name = "Isamaru, Hound of Konda"
	raw := `{"name":"snapshot-commander-damage","format":"commander","cr":["903.10a"],"why":"an unblocked commander attack",
"setup":{"p0":{"battlefield":["` + name + `"],"commanders":["` + name + `"]},"p1":{}},
"steps":[{"op":"attack","seat":0,"attackers":["p0:` + name + `"],"defender":"p1"},
         {"op":"pass","seat":0},{"op":"pass","seat":1},
         {"op":"pass","seat":0},{"op":"pass","seat":1}]}`
	run, fails := runOracleCommander(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	last := run.snaps[len(run.snaps)-1]
	p1 := last.Players[1]
	if got, want := p1.CommanderDamage, map[string]int32{"p0:" + name: 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("post-combat p1 CommanderDamage = %v, want %v", got, want)
	}
	if p1.Life != 38 {
		t.Fatalf("post-combat p1 life = %d, want 38", p1.Life)
	}
	if got := last.Players[0].CommanderDamage; got != nil {
		t.Fatalf("p0 CommanderDamage = %v, want nil", got)
	}
	if got := p1.CommanderCasts; got != nil {
		t.Fatalf("p1 CommanderCasts = %v, want nil", got)
	}
}

// TestOracleSnapshotConstructedHasNoCommanderKeys: a constructed scenario's
// snapshot JSON is unchanged: neither Commander-only key appears.
func TestOracleSnapshotConstructedHasNoCommanderKeys(t *testing.T) {
	const raw = `{"name":"snapshot-constructed","why":"a constructed game has no commander tallies",
"setup":{"p0":{"battlefield":["Isamaru, Hound of Konda"]}},"steps":[]}`
	run, fails := runOracleCommander(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	if len(run.snaps) == 0 {
		t.Fatal("precondition: the setup snapshot was not recorded")
	}
	b, err := json.Marshal(run.snaps)
	if err != nil {
		t.Fatalf("marshal snapshots: %v", err)
	}
	for _, key := range []string{"commander_casts", "commander_damage"} {
		if strings.Contains(string(b), key) {
			t.Fatalf("constructed snapshot JSON contains %q:\n%s", key, b)
		}
	}
}
