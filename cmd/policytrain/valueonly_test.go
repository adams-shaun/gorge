package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/view"
)

// The pn17 value-only tests: tiny inline synthetic state/outcome dumps (no
// Forge script is read), each fixture built and asserted on before the
// assertion that depends on it.

// outcomeFor is the fixture's outcome rule: the advantage side (even j)
// wins, so the life signal in the encoding separates the classes and every
// turn bucket holds both classes regardless of which seeds are held out.
func outcomeFor(j int) float64 {
	if j%2 == 0 {
		return 1
	}
	return 0
}

// dumpView builds one record's redacted view: the viewer (seat 0) and one
// opponent, whose life totals carry the outcome signal. advantage picks the
// side.
func dumpView(turn int32, advantage bool) view.View {
	me, opp := int32(25), int32(15)
	if !advantage {
		me, opp = 15, 25
	}
	return view.View{
		Viewer: 0, Visibility: "seat", Turn: turn, Round: turn,
		Phase: "main", Step: "main", Active: 0, Priority: 0,
		Players: []view.PlayerView{
			{ID: 0, Name: "me", Life: me, Hand: []view.CardView{}, Battlefield: []view.CardView{}, Graveyard: []view.CardView{}, Exile: []view.CardView{}},
			{ID: 1, Name: "opp", Life: opp, Battlefield: []view.CardView{}, Graveyard: []view.CardView{}, Exile: []view.CardView{}},
		},
	}
}

// writeStateDump writes nSeeds × recsPerSeed state/outcome records (record
// j of seed k: turn j+1, advantage on even j) and returns the path. Every
// seed is distinct — the split unit — and every record carries diagnostic
// extras (the opponent's hand and both libraries' tops) so the diagnostic
// feature sets have something to read.
func writeStateDump(t *testing.T, nSeeds, recsPerSeed int, withExtras bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "states.jsonl")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for k := 0; k < nSeeds; k++ {
		seed := uint64(9000 + k)
		for j := 0; j < recsPerSeed; j++ {
			turn := int32(j + 1)
			rec := policynet.StateRecord{
				RecordType: policynet.StateDumpRecordType, SchemaVersion: policynet.StateDumpSchemaVersion,
				Pair: "dump-vs-dump", GameIndex: k, Seed: seed, Sequence: uint64(j),
				Seat: 0, Turn: turn,
				Outcome:      outcomeFor(j),
				OutcomeKnown: true,
			}
			vb, err := json.Marshal(dumpView(turn, j%2 == 0))
			if err != nil {
				t.Fatal(err)
			}
			rec.View = vb
			if withExtras {
				rec.Extras = &policynet.LabelExtras{
					DiagOppHand: []view.CardView{
						{ID: 100, Name: fmt.Sprintf("Dump Charm %d", j), Types: "Instant", ManaCost: "1 U"},
					},
					DiagOppLibraryTop: []string{fmt.Sprintf("Opp Top %d", j)},
					DiagOwnLibraryTop: []string{fmt.Sprintf("Own Top %d", j)},
				}
			}
			if err := enc.Encode(rec); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// valueDump loads the fixture under fs and asserts the loader's counts (the
// input precondition every trainer test below leans on).
func valueDump(t *testing.T, path string, fs policynet.FeatureSet, n int) []policynet.StateExample {
	t.Helper()
	recs, stats, err := policynet.LoadStateOutcome(path, fs)
	if err != nil {
		t.Fatalf("LoadStateOutcome(%s): %v", fs, err)
	}
	if stats.Records != n || stats.Loaded != n || stats.Skipped != 0 {
		t.Fatalf("dump load: %d records, %d loaded, %d skipped, want %d/%d/0", stats.Records, stats.Loaded, stats.Skipped, n, n)
	}
	return recs
}

func valueOnlyCfg(seed int64) ValueOnlyConfig {
	return ValueOnlyConfig{Epochs: 60, Batch: 16, LR: 0.5, Seed: seed, Holdout: 0.3,
		Embed: 8, Hidden: 8, ValueHidden: 8, Clip: 1}
}

// TestValueOnlyTrainsAndReportsBuckets is the mode's core gate: the training
// BCE comes down, the holdout log loss beats the base-rate predictor, the
// readout's turn buckets are exactly 1-3 / 4-6 / 7+ and every bucket is
// populated with a discriminating AUC, and the whole run is deterministic
// from its seed. Preconditions asserted: the fixture has the distinct seeds
// the split needs and both outcome classes in every bucket.
func TestValueOnlyTrainsAndReportsBuckets(t *testing.T) {
	path := writeStateDump(t, 10, 8, true)
	recs := valueDump(t, path, policynet.FeaturesMZ, 80)
	seeds := map[uint64]bool{}
	for _, r := range recs {
		seeds[r.Seed] = true
	}
	if len(seeds) != 10 {
		t.Fatalf("fixture precondition: %d distinct seeds, want 10 (the split unit)", len(seeds))
	}
	classes := map[int]map[float64]bool{}
	for _, r := range recs {
		b := valueOnlyTurnBucket(r.Turn)
		if classes[b] == nil {
			classes[b] = map[float64]bool{}
		}
		classes[b][r.Outcome] = true
	}
	for b, cs := range classes {
		if !cs[0] || !cs[1] {
			t.Fatalf("fixture precondition: bucket %d lacks an outcome class (%v)", b, cs)
		}
	}

	cfg := valueOnlyCfg(3)
	res, err := TrainValueOnly(recs, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.TrainN+res.HoldoutN != 80 || res.HoldoutN == 0 || res.HoldoutSeeds == 0 {
		t.Fatalf("split: train %d + holdout %d (%d seed blocks) of 80", res.TrainN, res.HoldoutN, res.HoldoutSeeds)
	}
	first, final := res.Epochs[0], res.Epochs[len(res.Epochs)-1]
	if final.TrainLogLoss >= first.TrainLogLoss {
		t.Fatalf("train BCE did not fall: %.6f -> %.6f", first.TrainLogLoss, final.TrainLogLoss)
	}
	if final.TrainLogLoss > 0.5*first.TrainLogLoss {
		t.Fatalf("train BCE only %.1fx better (%.6f -> %.6f)", first.TrainLogLoss/final.TrainLogLoss, first.TrainLogLoss, final.TrainLogLoss)
	}
	vs := res.Holdout
	t.Logf("holdout: log loss %.4f brier %.4f | base rate %.3f log loss %.4f brier %.4f",
		vs.LogLoss, vs.Brier, vs.BaseRate, vs.BaseLogLoss, vs.BaseBrier)
	if !(vs.LogLoss < vs.BaseLogLoss) || !(vs.Brier < vs.BaseBrier) {
		t.Fatalf("value head did not beat the base rate: log loss %.4f vs %.4f, brier %.4f vs %.4f", vs.LogLoss, vs.BaseLogLoss, vs.Brier, vs.BaseBrier)
	}
	if len(res.ByTurn) != 3 {
		t.Fatalf("%d turn buckets, want 3", len(res.ByTurn))
	}
	for i, want := range []string{"1-3", "4-6", "7+"} {
		b := res.ByTurn[i]
		if b.Turns != want {
			t.Fatalf("bucket %d is %q, want %q", i, b.Turns, want)
		}
		if b.N == 0 {
			t.Fatalf("bucket %s is empty — the fixture's turns must populate every bucket", b.Turns)
		}
		t.Logf("turns %-5s n %d log loss %.4f (base rate %.3f, base ll %.4f) AUC %.4f",
			b.Turns, b.N, b.LogLoss, b.BaseRate, b.BaseLogLoss, b.AUC)
		if !(b.LogLoss < b.BaseLogLoss) {
			t.Fatalf("bucket %s: log loss %.4f did not beat its base rate's %.4f", b.Turns, b.LogLoss, b.BaseLogLoss)
		}
		if b.AUC <= 0.5 {
			t.Fatalf("bucket %s AUC %.4f does not discriminate (the life signal is in the encoding)", b.Turns, b.AUC)
		}
	}

	// Determinism: same seed, same corpus ⇒ the same split, the same epoch
	// statistics and the same value head.
	res2, err := TrainValueOnly(recs, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(res.Epochs) != fmt.Sprint(res2.Epochs) || *res.Holdout != *res2.Holdout ||
		fmt.Sprint(res.ByTurn) != fmt.Sprint(res2.ByTurn) {
		t.Fatal("value-only training is not reproducible from its seed")
	}
	if res.Model.Value(recs[0].State) != res2.Model.Value(recs[0].State) {
		t.Fatal("two same-seed runs trained different value heads")
	}
}

// TestValueOnlySplitBySeedNoLeak pins the split's contract: the seed-block
// holdout is deterministic from the run seed, holds whole seed blocks, and
// NO seed appears on both sides (a per-example split on this fixture WOULD
// leak, which the test also demonstrates). Precondition: the fixture's
// seeds are distinct.
func TestValueOnlySplitBySeedNoLeak(t *testing.T) {
	path := writeStateDump(t, 8, 6, false)
	recs := valueDump(t, path, policynet.FeaturesMZ, 48)
	distinct := map[uint64]bool{}
	for _, r := range recs {
		distinct[r.Seed] = true
	}
	if len(distinct) != 8 {
		t.Fatalf("fixture precondition: %d distinct seeds, want 8", len(distinct))
	}
	for _, seed := range []int64{1, 4, 9} {
		rng := valueSplitRNG(seed)
		sp := splitBySeedBlock(recs, 0.3, rng)
		again := splitBySeedBlock(recs, 0.3, valueSplitRNG(seed))
		if fmt.Sprint(sp.hold) != fmt.Sprint(again.hold) || fmt.Sprint(sp.train) != fmt.Sprint(again.train) {
			t.Fatalf("seed %d: two runs split differently", seed)
		}
		trainSeeds, holdSeeds := map[uint64]bool{}, map[uint64]bool{}
		for _, ix := range sp.train {
			trainSeeds[recs[ix].Seed] = true
		}
		for _, ix := range sp.hold {
			holdSeeds[recs[ix].Seed] = true
		}
		if len(holdSeeds) == 0 {
			t.Fatalf("seed %d: empty holdout", seed)
		}
		for s := range holdSeeds {
			if trainSeeds[s] {
				t.Fatalf("seed %d: seed block %d is on both sides — leaked", seed, s)
			}
		}
		if float64(len(sp.hold)) < 0.3*float64(len(recs)) {
			t.Fatalf("seed %d: held %d records < the 0.3 fraction of %d", seed, len(sp.hold), len(recs))
		}
		t.Logf("seed %d: %d train / %d holdout records across %d held seed blocks", seed, len(sp.train), len(sp.hold), len(holdSeeds))
	}
	// The defect the seed block exists for: the per-record split shares
	// seeds across sides on this fixture.
	byEx := splitByRecord(recs, 0.3, valueSplitRNG(1))
	held := map[uint64]bool{}
	for _, ix := range byEx.hold {
		held[recs[ix].Seed] = true
	}
	shared := false
	for _, ix := range byEx.train {
		if held[recs[ix].Seed] {
			shared = true
			break
		}
	}
	if !shared {
		t.Fatal("expected a per-record split to share a seed across sides on this fixture")
	}
	// Through the trainer: a run whose split is by seed block trains, and a
	// holdout that would take every seed block is refused.
	res, err := TrainValueOnly(recs, valueOnlyCfg(2))
	if err != nil {
		t.Fatal(err)
	}
	if res.HoldoutSeeds == 0 {
		t.Fatal("trainer reported no held-out seed block")
	}
	cfg := valueOnlyCfg(2)
	cfg.Holdout = 0.99
	if _, err := TrainValueOnly(recs, cfg); err != nil && !strings.Contains(err.Error(), "every seed block") {
		t.Fatalf("8 seeds at holdout 0.99: %v (want the every-seed-block refusal)", err)
	}
}

// TestStateOutcomeLoaderEncodesAllFeatureSets proves the fixture's records
// re-encode under all three requested feature sets and that the diagnostic
// encodings actually differ from the redacted one (the precondition the
// "diagnostic path" claim rests on); the loader fails closed on a
// diagnostic set over a record with no extras and on a foreign record type.
func TestStateOutcomeLoaderEncodesAllFeatureSets(t *testing.T) {
	path := writeStateDump(t, 4, 3, true)
	mz := valueDump(t, path, policynet.FeaturesMZ, 12)
	opp := valueDump(t, path, policynet.FeaturesMZOppHand, 12)
	orc := valueDump(t, path, policynet.FeaturesMZOracle, 12)
	same := func(a, b policynet.State) bool { return fmt.Sprint(a.Sparse) == fmt.Sprint(b.Sparse) }
	if same(mz[0].State, opp[0].State) {
		t.Fatal("fixture precondition failed: mz-opphand's encoding equals mz's (the diag tokens are absent — the diagnostic path would be vacuous)")
	}
	if same(opp[0].State, orc[0].State) {
		t.Fatal("fixture precondition failed: mz-oracle's encoding equals mz-opphand's (the library tops are absent)")
	}
	for i := range mz {
		if mz[i].Outcome != opp[i].Outcome || mz[i].Turn != opp[i].Turn || mz[i].Seed != opp[i].Seed {
			t.Fatalf("record %d's metadata differs across feature sets", i)
		}
	}

	// No extras + a diagnostic set fails closed.
	noExtras := writeStateDump(t, 1, 2, false)
	if _, _, err := policynet.LoadStateOutcome(noExtras, policynet.FeaturesMZOppHand); err == nil {
		t.Fatal("a diagnostic feature set over a record with no extras was accepted")
	}
	// Redacted sets do not need extras.
	if recs, _, err := policynet.LoadStateOutcome(noExtras, policynet.FeaturesMZ); err != nil || len(recs) != 2 {
		t.Fatalf("mz over an extras-less dump: %d records, err %v", len(recs), err)
	}
	// A foreign record type and a foreign schema are hard errors.
	bad := filepath.Join(t.TempDir(), "bad.jsonl")
	os.WriteFile(bad, []byte("{\"record_type\":\"label-v1\",\"schema_version\":1}\n"), 0o644)
	if _, _, err := policynet.LoadStateOutcome(bad, policynet.FeaturesMZ); err == nil {
		t.Fatal("a label record was accepted by the state-dump loader")
	}
	os.WriteFile(bad, []byte(fmt.Sprintf("{\"record_type\":%q,\"schema_version\":2}\n", policynet.StateDumpRecordType)), 0o644)
	if _, _, err := policynet.LoadStateOutcome(bad, policynet.FeaturesMZ); err == nil {
		t.Fatal("schema_version 2 was accepted by the state-dump loader")
	}
	// A stalled game's record (outcome_known false) is skipped, not an error.
	os.WriteFile(bad, []byte("{\"record_type\":\"state-v1\",\"schema_version\":1,\"seed\":1,\"view\":{\"viewer\":0}}\n"), 0o644)
	recs, stats, err := policynet.LoadStateOutcome(bad, policynet.FeaturesMZ)
	if err != nil || len(recs) != 0 || stats.Skipped != 1 {
		t.Fatalf("an outcome-less record: %d records, skipped %d, err %v", len(recs), stats.Skipped, err)
	}
}

// TestValueOnlyCheckpointLoadsForConsumer drives the CLI end to end: an mz
// run writes an ORDINARY policynet checkpoint whose value head the pn17
// consumer reads (policynet.LoadCheckpointFile + Model.Value), whose policy
// read is deliberately zero, and whose bytes are reproducible from the
// seed; the diagnostic encoding stays measurement-only and writes nothing;
// the mode's flag exclusivity is enforced.
func TestValueOnlyCheckpointLoadsForConsumer(t *testing.T) {
	path := writeStateDump(t, 6, 8, true)
	base := []string{"-value-corpus", path, "-epochs", "10", "-embed", "8", "-hidden", "8",
		"-batch", "16", "-lr", "0.5", "-value-hidden", "8", "-seed", "5", "-holdout", "0.25"}

	// The redacted mz run checkpoints.
	out1 := filepath.Join(t.TempDir(), "model.bin")
	var stdout, stderr bytes.Buffer
	code := run(append(append([]string{}, base...), "-out", out1, "-features", "mz"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mz run() = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "value-only training") || !strings.Contains(stdout.String(), "holdout by turn") || !strings.Contains(stdout.String(), "  1-3   ") {
		t.Fatalf("stdout missing the value-only readout:\n%s", stdout.String())
	}
	m, err := policynet.LoadCheckpointFile(out1)
	if err != nil {
		t.Fatalf("the value-only checkpoint does not load through the ordinary loader (the pn17 consumer's read): %v", err)
	}
	if m.Features != policynet.FeaturesMZ || !m.HasValue() || m.ValueHidden != 8 {
		t.Fatalf("checkpoint features %s value-hidden %d, want mz/8", m.Features, m.ValueHidden)
	}
	if len(m.HidW) == 0 {
		t.Fatal("checkpoint has no policy geometry")
	}
	for _, v := range m.HidW {
		if v != 0 {
			t.Fatal("the value-only checkpoint's policy read is not zeroed")
		}
	}
	if m.OutB != 0 || m.ResidualW != 0 {
		t.Fatalf("policy bias %g residual %g, want 0/0", m.OutB, m.ResidualW)
	}
	// The consumer reads the value head off the SAME records: the checkpoint's
	// V(s) matches the trained trainer's, and it learned (not 0.5 everywhere).
	recs := valueDump(t, path, policynet.FeaturesMZ, 48)
	res, err := TrainValueOnly(recs, ValueOnlyConfig{Epochs: 10, Batch: 16, LR: 0.5, Seed: 5, Holdout: 0.25, Embed: 8, Hidden: 8, ValueHidden: 8, Clip: 1})
	if err != nil {
		t.Fatal(err)
	}
	zeroPolicyHead(res.Model)
	anyNonHalf := false
	for _, r := range recs {
		got, want := m.Value(r.State), res.Model.Value(r.State)
		if got != want {
			t.Fatalf("consumer V(s) %.6f != the trained head's %.6f on record seed %d turn %d", got, want, r.Seed, r.Turn)
		}
		if got != 0.5 {
			anyNonHalf = true
		}
	}
	if !anyNonHalf {
		t.Fatal("the value head predicts 0.5 on every record — it learned nothing (fixture signal broken)")
	}
	// Determinism: the same flags again write byte-identical checkpoint bytes.
	var stdout2 bytes.Buffer
	out2 := filepath.Join(t.TempDir(), "model.bin")
	if code := run(append(append([]string{}, base...), "-out", out2, "-features", "mz"), &stdout2, &stderr); code != 0 {
		t.Fatalf("second mz run() = %d, stderr: %s", code, stderr.String())
	}
	a, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("two same-seed value-only runs wrote different checkpoint bytes")
	}

	// The diagnostic encoding is measurement-only: no checkpoint file (a
	// fresh -out path, so the stat below reads THIS run's output).
	var stdout3 bytes.Buffer
	outDiag := filepath.Join(t.TempDir(), "diag.bin")
	if code := run(append(append([]string{}, base...), "-out", outDiag, "-features", "mz-opphand"), &stdout3, &stderr); code != 0 {
		t.Fatalf("mz-opphand run() = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout3.String(), "measurement only: no checkpoint written") {
		t.Fatalf("diagnostic run's stdout lacks the measurement-only line:\n%s", stdout3.String())
	}
	if _, err := os.Stat(outDiag); err == nil {
		t.Fatal("the diagnostic run wrote a checkpoint")
	}

	// Flag exclusivity.
	if code := run([]string{"-value-corpus", path, "-corpus", "x.jsonl", "-out", out1}, &stdout, &stderr); code == 0 {
		t.Fatal("-value-corpus with -corpus accepted")
	}
	if code := run([]string{"-value-corpus", path, "-out", out1, "-holdout-by", "game"}, &stdout, &stderr); code == 0 {
		t.Fatal("-value-corpus with -holdout-by accepted")
	}
}

// TestPolicyCheckpointBytesUnchanged pins the ordinary path: a policy-mode
// CLI run writes a schema-3 checkpoint with a TRAINED (non-zero) policy
// read and no value head, byte-identically across two runs — the value-only
// mode must not have moved a byte of it. Precondition asserted: the policy
// head's weights are actually non-zero, so "not zeroed" is a real check.
func TestPolicyCheckpointBytesUnchanged(t *testing.T) {
	path := writeJSONLCorpus(t, 48)
	args := func(out string) []string {
		return []string{"-corpus", path, "-out", out,
			"-epochs", "5", "-embed", "8", "-hidden", "8", "-batch", "8",
			"-seed", "9", "-holdout", "0.2"}
	}
	out1 := filepath.Join(t.TempDir(), "policy.bin")
	var stdout, stderr bytes.Buffer
	if code := run(args(out1), &stdout, &stderr); code != 0 {
		t.Fatalf("policy run() = %d, stderr: %s", code, stderr.String())
	}
	out2 := filepath.Join(t.TempDir(), "policy.bin")
	if code := run(args(out2), &stdout, &stderr); code != 0 {
		t.Fatalf("second policy run() = %d, stderr: %s", code, stderr.String())
	}
	a, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("two same-seed policy runs write different checkpoint bytes")
	}
	m, err := policynet.LoadCheckpointFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	if m.ValueHidden != 0 || m.HasValue() {
		t.Fatalf("policy checkpoint carries value hidden %d", m.ValueHidden)
	}
	nonZero := 0
	for _, v := range m.HidW {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatal("fixture precondition failed: the policy head trained to all-zero (the not-zeroed check would be vacuous)")
	}
	t.Logf("policy checkpoint: %d bytes, %d non-zero policy weights, value-hidden %d", len(a), nonZero, m.ValueHidden)
}
