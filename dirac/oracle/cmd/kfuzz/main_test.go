package main

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

func TestResolveKernel(t *testing.T) {
	t.Setenv("DIRAC_POOL_BLB", "")
	t.Setenv("DIRAC_POOL_FDN", "")
	t.Setenv("KORACLE_BLB_DECKS", "")
	pools := t.TempDir()
	t.Setenv("GORGE_POOLS", pools)
	fdnDecks := filepath.Join(pools, "fdn", "decks")
	k, s, d, err := resolveKernel("blb", "fdn", fdnDecks, map[string]bool{})
	if err != nil || k != "fdn" || s != "blb" || d != filepath.Join(pools, "blb", "decks") {
		t.Errorf("blb default: %q %q %q %v", k, s, d, err)
	}
	if _, _, d, err = resolveKernel("blb", "fdn", "/x", map[string]bool{"decks": true}); err != nil || d != "/x" {
		t.Errorf("explicit -decks: %q %v", d, err)
	}
	if _, _, _, err = resolveKernel("blb", "dsk", fdnDecks, map[string]bool{"set": true}); err == nil {
		t.Error("blb with -set dsk: want an error")
	}
	if _, s, _, err = resolveKernel("blb", "blb", fdnDecks, map[string]bool{"set": true}); err != nil || s != "blb" {
		t.Errorf("blb with -set blb: %q %v", s, err)
	}
	for _, in := range []string{"fdn", "gorge", "nosuch"} {
		k, s, d, err := resolveKernel(in, "fdn", fdnDecks, map[string]bool{})
		if err != nil || k != in || s != "fdn" || d != fdnDecks {
			t.Errorf("%s: %q %q %q %v", in, k, s, d, err)
		}
	}
	// the fdn default -decks is the registry's fdn inputs
	if p, _ := oracle.PoolByName("fdn"); filepath.Join(p.Inputs, "decks") != fdnDecks || oracle.PoolDecks("fdn") != fdnDecks {
		t.Errorf("fdn inputs %q, PoolDecks %q, want decks %q", p.Inputs, oracle.PoolDecks("fdn"), fdnDecks)
	}
}

// mutatedPair is the FDN pair (seed 1) that diverges under mutateKernel, plays
// clean without it, and holds no card refused by name.
const mutatedPair = 386

// mutateKernel makes the kernel deal one damage too many on the effects that
// deal it, which gives a command-line test a game the two sides disagree on.
func mutateKernel(on bool) { dirac.DebugDmgAmountPlusOne = on }

// runKFuzz drives run() with args, resetting the global flag set (testing has
// already parsed the process flags) and capturing its stdout/stderr.
func runKFuzz(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	oldFlag, oldArgs := flag.CommandLine, os.Args
	flag.CommandLine = flag.NewFlagSet("kfuzz", flag.ContinueOnError)
	os.Args = append([]string{"kfuzz"}, args...)
	defer func() { flag.CommandLine, os.Args = oldFlag, oldArgs }()
	return captureStd(t, run)
}

// captureStd runs f with stdout and stderr redirected to files and returns its
// code and what it wrote.
func captureStd(t *testing.T, f func() int) (code int, stdout, stderr string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	dir := t.TempDir()
	outF, err := os.CreateTemp(dir, "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errF, err := os.CreateTemp(dir, "stderr")
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outF, errF
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	code = f()
	os.Stdout, os.Stderr = oldOut, oldErr
	outF.Close()
	errF.Close()
	ob, _ := os.ReadFile(outF.Name())
	eb, _ := os.ReadFile(errF.Name())
	return code, string(ob), string(eb)
}

// TestKFuzzFDNReproFlags covers the fdn/vanilla divergence-reporting flags at
// the CLI: -out writes the first divergence's repro.json (and a clean run
// writes nothing), -repro reruns a repro under either lead, -replay is routed
// before the kernel switch, and all three report errors with exit 1 instead of
// starting a sweep.
func TestKFuzzFDNReproFlags(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	fdnDecks := oracle.PoolDecks("fdn")
	if _, err := os.Stat(fdnDecks); err != nil {
		t.Skipf("FDN decks absent (set GORGE_POOLS): %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(fdnDecks), "pool.tsv")); err != nil {
		t.Skipf("FDN pool absent: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	diverging := []string{"-kernel", "fdn", "-first-pair", strconv.Itoa(mutatedPair), "-pairs", "1", "-seed", "1",
		"-keep-going", "-autopay-surface", "-shrink", "0", "-allow-unverified"}

	mutateKernel(true)
	defer mutateKernel(false)
	outDir := filepath.Join(t.TempDir(), "repro-out")
	code, stdout, stderr := runKFuzz(t, append(diverging, "-out", outDir)...)
	if code != 1 {
		t.Fatalf("-out run: exit %d, want 1 (stdout %q stderr %q)", code, stdout, stderr)
	}
	reproPath := filepath.Join(outDir, "repro.json")
	rep, err := oracle.ReadRepro(reproPath)
	if err != nil {
		t.Fatalf("-out: %v", err)
	}
	if len(rep.Actions) == 0 {
		t.Fatal("-out: repro carries no action prefix")
	}
	if rep.Kernel != "set fdn" || rep.Surface != "autopay" {
		t.Fatalf("-out: repro records kernel %q surface %q, want \"set fdn\" and \"autopay\"", rep.Kernel, rep.Surface)
	}
	// replayed against the default gorge self-check, or on the full surface, it
	// is another game: an error, not "no divergence"
	for _, args := range [][]string{{"-repro", reproPath}, {"-kernel", "fdn", "-repro", reproPath, "-surface", "full"}} {
		code, stdout, stderr = runKFuzz(t, args...)
		if code != 2 || strings.Contains(stdout, "no divergence") {
			t.Fatalf("%v: exit %d, want 2 (stdout %q stderr %q)", args, code, stdout, stderr)
		}
	}

	mutateKernel(false)
	cleanOut := filepath.Join(t.TempDir(), "clean-out")
	code, stdout, stderr = runKFuzz(t, append(diverging, "-out", cleanOut)...)
	if code != 0 {
		t.Fatalf("clean run: exit %d, want 0 (stdout %q stderr %q)", code, stdout, stderr)
	}
	if _, err := os.Stat(cleanOut); !os.IsNotExist(err) {
		t.Fatalf("clean run wrote %s (stat err %v)", cleanOut, err)
	}

	mutateKernel(true)
	for _, lead := range []string{"gorge", "kernel"} {
		code, stdout, stderr = runKFuzz(t, "-kernel", "fdn", "-repro", reproPath, "-lead", lead,
			"-autopay-surface")
		if code != 1 {
			t.Fatalf("fdn -repro -lead %s: exit %d, want 1 (stdout %q stderr %q)", lead, code, stdout, stderr)
		}
		if !strings.Contains(stdout, "divergence at decision") {
			t.Fatalf("fdn -repro -lead %s: no divergence report (stdout %q stderr %q)", lead, stdout, stderr)
		}
		if strings.Contains(stdout, "fdn:") {
			t.Fatalf("fdn -repro -lead %s: swept instead of replaying (stdout %q)", lead, stdout)
		}
	}

	mutateKernel(false)

	// A missing -repro must fail before either path sweeps.
	code, stdout, stderr = runKFuzz(t, "-kernel", "fdn", "-repro", missing, "-autopay-surface")
	if code != 1 || !strings.Contains(stderr, "kfuzz:") || strings.Contains(stdout, "fdn:") {
		t.Fatalf("fdn -repro missing: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runKFuzz(t, "-kernel", "vanilla", "-repro", missing)
	if code != 1 || !strings.Contains(stderr, "kfuzz:") || strings.Contains(stdout, "vanilla:") {
		t.Fatalf("vanilla -repro missing: exit %d stdout %q stderr %q", code, stdout, stderr)
	}

	// -replay is routed before the kernel switch for every kernel.
	for _, k := range []string{"fdn", "vanilla", "gorge", "gorge-api"} {
		code, stdout, stderr = runKFuzz(t, "-kernel", k, "-replay", missing)
		if code != 1 || !strings.Contains(stderr, "kfuzz:") {
			t.Fatalf("%s -replay missing: exit %d stdout %q stderr %q", k, code, stdout, stderr)
		}
		if strings.Contains(stdout, "replay:") || strings.Contains(stdout, "fdn:") || strings.Contains(stdout, "vanilla:") {
			t.Fatalf("%s -replay missing: started work (stdout %q)", k, stdout)
		}
	}
}

// TestReproRunChecksSideAndSurface: a repro replays against the side and on the
// surface it was found on. A repro found on a synthetic pool and replayed with no
// -synth plays gorge against itself, so it read "no divergence" for a game that
// diverges; now that is an error naming the flags, and the recorded surface
// replaces the default one.
// TestStampReproRecordsSynthPool: a synth run's repro names its pool, so the
// lockstep tables' glob (which matches on Repro.Pool) picks it up.
func TestStampReproRecordsSynthPool(t *testing.T) {
	defer func(k string) { runKernel = k }(runKernel)
	for _, c := range []struct{ kernel, pool string }{
		{"synth std-ecl", "std-ecl"},
		{"set blb", ""},
		{"gorge", ""},
		{"vanilla", ""},
	} {
		runKernel = c.kernel
		r := stampRepro(&oracle.Repro{})
		if r.Kernel != c.kernel || r.Pool != c.pool {
			t.Errorf("%s: kernel %q, pool %q; want %q, %q", c.kernel, r.Kernel, r.Pool, c.kernel, c.pool)
		}
	}
}

func TestReproRunChecksSideAndSurface(t *testing.T) {
	path := "found.repro.json"
	rep := &oracle.Repro{Kernel: "synth std-trk", Surface: "autopay"}
	defer func(k string) { runKernel = k }(runKernel)
	for _, c := range []struct {
		run, surface string
		given        bool
		want         string // the surface the replay plays, "" for an error
		wantErr      string
	}{
		{"gorge", "full", false, "", "replay it with -synth std-trk"},
		{"synth std-trc", "full", false, "", "replay it with -synth std-trk"},
		{"synth std-trk", "full", false, "autopay", ""},
		{"synth std-trk", "full", true, "", "played on the autopay surface"},
		{"synth std-trk", "autopay", true, "autopay", ""},
	} {
		runKernel = c.run
		got, err := reproRun(rep, path, c.surface, c.given)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s -surface %s (given %v): err %v, want one saying %q", c.run, c.surface, c.given, err, c.wantErr)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s -surface %s (given %v): %q, %v; want %q", c.run, c.surface, c.given, got, err, c.want)
		}
	}
	// a repro written before either was recorded runs as the command line says
	runKernel = "gorge"
	if got, err := reproRun(&oracle.Repro{}, path, "full", false); err != nil || got != "full" {
		t.Errorf("unrecorded repro: %q, %v; want full", got, err)
	}
	if got := kernelFlags("set blb"); got != "-kernel fdn -set blb" {
		t.Errorf("kernelFlags(set blb) = %q", got)
	}
}

// TestFuzzPlaysTheCommittedSlice: with no pool on disk, a pool run plays the
// committed slice, and asking for pairs it does not hold is an error naming the way
// to get them rather than a silently different run.
func TestFuzzPlaysTheCommittedSlice(t *testing.T) {
	if _, err := cardset.CorpusDir(); err != nil {
		t.Skip(err)
	}
	cardsettest.Set(t, "blb")
	t.Setenv("GORGE_POOLS", t.TempDir())
	code, stdout, stderr := runKFuzz(t, "-kernel", "blb", "-first-pair", "0", "-pairs", "3", "-seed", "1", "-shrink", "0", "-keep-going")
	if code != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "blb") {
		t.Errorf("no verdict line in %q", stdout)
	}
	code, _, stderr = runKFuzz(t, "-kernel", "blb", "-pairs", "3", "-seed", "7")
	if code == 0 || !strings.Contains(stderr, "GORGE_POOLS") {
		t.Errorf("seed outside the slice: exit %d, stderr %q", code, stderr)
	}
}
