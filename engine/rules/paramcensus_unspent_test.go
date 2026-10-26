package rules

import "testing"

// TestParamCensusReadsUnspentManaParams: unspentManaKeep's per-line fold reads
// ValidPlayer$ and ManaType$ (CR 500.4's "unless an effect says otherwise";
// Electro, Assaulting Battery and The Last Agni Kai's Effect-delivered grant).
// The fold is a named method so the census classifies both reads; a closure
// hides them and re-labels the stat:UnspentMana carriers.
func TestParamCensusReadsUnspentManaParams(t *testing.T) {
	t.Parallel()
	_, d := measureParamCensus(t, nil)
	reads := d.stat["UnspentMana"]
	if reads == nil {
		t.Fatal("stat:UnspentMana is not a derived bucket -- the census cannot see the fold at all")
	}
	for _, key := range []string{"ValidPlayer", "ManaType"} {
		if !reads[key] {
			t.Errorf("stat:UnspentMana read set %v lacks %s -- unspentManaFold's read is invisible to the census", reads, key)
		}
	}
}

// TestParamCensusSacrificeModeIsInert pins the two scoped classifications:
// Sacrifice's Mode$ is a Discard-family spelling Forge's SacrificeEffect never
// reads (the sacrificing player already chooses), and Token's
// LockTokenScript$ opts out of a source text-change gorge never applies.
// api:Discard.Mode stays unignored -- Discard really reads it.
func TestParamCensusSacrificeModeIsInert(t *testing.T) {
	t.Parallel()
	_, d := measureParamCensus(t, nil)
	if !ignoredParam("api:Sacrifice", "Mode") {
		t.Error("api:Sacrifice.Mode is not classified ignored -- Lord Xander and Rush of Dread stay false positives")
	}
	if !ignoredParam("api:Token", "LockTokenScript") {
		t.Error("api:Token.LockTokenScript is not classified ignored -- Overlord of the Hauntwoods stays a false positive")
	}
	if d.api["Discard"] == nil || !d.api["Discard"]["Mode"] {
		t.Fatal("precondition: api:Discard's Mode read is not derived -- the sibling check below measures nothing")
	}
	if ignoredParam("api:Discard", "Mode") {
		t.Error("api:Discard.Mode is classified ignored -- Discard's real Mode read is suppressed")
	}
}
