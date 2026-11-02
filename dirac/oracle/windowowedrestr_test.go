package oracle

import "testing"

// TestWindowOwedRestrictedLockstep: Moonrise Cleric's {W/B}{W/B} answered black with
// white floating. gorge's ability ask lists Mudflat Village's restricted {B} first
// because the pip owes it, so the generic stays for the pool and no Swamp is tapped.
func TestWindowOwedRestrictedLockstep(t *testing.T) {
	c, _ := testPoolCorpus(t, "blb")
	const seed = 16256398713181607051
	cfg := Config{ArrangeRestOrder: true}
	m, tbl, err := BLBMatchup(c, "BLB_top_31752_WBpG", "BLB_top_30694_UR", 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := BLBSide(tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewGorgeSide(m, seed, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
	if res.Div != nil {
		t.Fatalf("diverged at %d: %s %v", res.Div.At, res.Div.What, res.Div.Diff)
	}
}
