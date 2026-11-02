package oracle

import (
	"path/filepath"
	"testing"
)

// TestReplayKernelGame records a game a (gorge-backed) kernel played alone
// and re-executes it in gorge: the seeded game reproduces exactly and its
// gorge log verifies through engine/replay; a game recorded with its chance
// transcript reproduces even under a different seed (the forced outcomes
// carry every shuffle); a tampered hash and a tampered action are reported
// at their own sequence numbers.
func TestReplayKernelGame(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	p := Pairings(pool, 1, 7)[0]
	m, err := c.Matchup(p.A, p.B, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{PassElision: true}
	k, err := GorgeKernel(cfg)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	g := RecordKernelGame(m, p.Seed, k, RandomPolicy(p.Seed), cfg, 0)
	path := filepath.Join(t.TempDir(), "game.json")
	if err := g.Write(path); err != nil {
		t.Fatal(err)
	}
	g2, err := ReadKernelGame(path)
	if err != nil {
		t.Fatal(err)
	}
	r := Replay(c.Reg, g2)
	if r.DivergedAt != -1 || r.Verified != nil {
		t.Fatalf("seeded replay: diverged at %d (%s: %s), verified %v", r.DivergedAt, r.Reason, r.Detail, r.Verified)
	}
	t.Logf("seeded: %d actions, %d gorge intents, %d events", r.Applied, len(r.Log.Intents), len(r.Log.Events))

	kc, err := RecordChance(cfg)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	gc := RecordKernelGame(m, p.Seed, kc, RandomPolicy(p.Seed), cfg, 0)
	if len(gc.Chance) == 0 {
		t.Fatal("no chance transcript recorded")
	}
	if len(gc.Actions) != len(g.Actions) || gc.Hashes[len(gc.Hashes)-1] != g.Hashes[len(g.Hashes)-1] {
		t.Fatalf("a recording engine played a different game: %d vs %d actions", len(gc.Actions), len(g.Actions))
	}
	gc.Seed ^= 0x5eed
	r = Replay(c.Reg, gc)
	if r.DivergedAt != -1 || r.Verified != nil {
		t.Fatalf("forced replay under another seed: diverged at %d (%s: %s), verified %v", r.DivergedAt, r.Reason, r.Detail, r.Verified)
	}
	t.Logf("forced: %d chance draws, %d actions", len(gc.Chance), r.Applied)

	bad := *g
	bad.Hashes = append([]uint64(nil), g.Hashes...)
	at := len(bad.Hashes) / 2
	bad.Hashes[at] ^= 1
	if r := Replay(c.Reg, &bad); r.DivergedAt > at || r.Reason != "projection" {
		t.Fatalf("tampered hash %d: diverged at %d (%s)", at, r.DivergedAt, r.Reason)
	}
	bad = *g
	bad.Actions = append([]Canon(nil), g.Actions...)
	bad.Actions[3] = Pack(ActCast, 200, RefNone, 0)
	if r := Replay(c.Reg, &bad); r.DivergedAt != 3 || r.Reason != "not_offered" {
		t.Fatalf("tampered action: diverged at %d (%s)", r.DivergedAt, r.Reason)
	}
}
