package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

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

// TestParamCensusReadsUnspentManaOnCorpusCards pins the two real carriers the
// set census named before cr-g184 (Electro, Assaulting Battery and The Last
// Agni Kai). TestParamCensusReadsUnspentManaParams pins the derived read set;
// this test pins the card-level walk, including The Last Agni Kai's
// Effect-delivered static reached through EachRawEffectChild. A closure in
// unspentManaKeep hides the ValidPlayer$/ManaType$ reads and re-labels both
// cards.
func TestParamCensusReadsUnspentManaOnCorpusCards(t *testing.T) {
	t.Parallel()
	_, d := measureParamCensus(t, nil)

	// Precondition: both cards' UnspentMana statics are reached even when the
	// fold's two reads are removed from a COPY of the read set. Do not delete
	// from d itself: measureParamCensus returns the memoised census shared by
	// every parallel census test.
	blind := &derivedReads{api: d.api, trig: d.trig, stat: map[string]map[string]bool{}, repl: d.repl}
	for mode, keys := range d.stat {
		cp := make(map[string]bool, len(keys))
		for k, v := range keys {
			cp[k] = v
		}
		blind.stat[mode] = cp
	}
	delete(blind.stat["UnspentMana"], "ValidPlayer")
	delete(blind.stat["UnspentMana"], "ManaType")

	reg := testutil.CorpusRegistry(t)
	want := []string{"param:stat:UnspentMana.ManaType", "param:stat:UnspentMana.ValidPlayer"}
	for _, name := range []string{"Electro, Assaulting Battery", "The Last Agni Kai"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("precondition: corpus lacks %q", name)
		}
		blindLabels := cardCensusLabels(c, blind, nil)
		blindSet := map[string]bool{}
		for _, l := range blindLabels {
			blindSet[l] = true
		}
		for _, l := range want {
			if !blindSet[l] {
				t.Fatalf("precondition: %q's UnspentMana static is not reached by the census walk; blind labels=%v", name, blindLabels)
			}
		}
		for _, l := range cardCensusLabels(c, d, nil) {
			if strings.HasPrefix(l, "param:stat:UnspentMana.") {
				t.Errorf("%q carries %q -- the census cannot see unspentManaFold's reads", name, l)
			}
		}
	}
}
