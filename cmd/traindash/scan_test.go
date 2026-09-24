package main

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const fixtureRoot = "testdata/root"

// scanAt scans the fixture with the clock set offset after arm1's stderr
// mtime (git does not preserve mtimes, so liveness is pinned relative to it).
func scanAt(t *testing.T, offset time.Duration) *Snapshot {
	t.Helper()
	fi, err := os.Stat(filepath.Join(fixtureRoot, "expa/runs/arm1.stderr"))
	if err != nil {
		t.Fatal(err)
	}
	s := NewScanner([]string{fixtureRoot})
	s.Now = func() time.Time { return fi.ModTime().Add(offset) }
	return s.Scan()
}

func findRun(t *testing.T, snap *Snapshot, id string) *Run {
	t.Helper()
	for _, e := range snap.Experiments {
		for _, r := range e.Runs {
			if r.ID == id {
				return r
			}
		}
	}
	t.Fatalf("run %q not discovered", id)
	return nil
}

func TestDiscovery(t *testing.T) {
	snap := scanAt(t, time.Minute)
	var ids, exps []string
	for _, e := range snap.Experiments {
		exps = append(exps, e.Name)
		for _, r := range e.Runs {
			ids = append(ids, r.ID)
		}
	}
	want := []string{"expa/runs/arm1", "expa/runs/arm2", "expb/cmp/run3"}
	sort.Strings(ids)
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("runs = %v, want %v (gotmp/ and scratch/ must be skipped)", ids, want)
	}
	if strings.Join(exps, ",") != "expa,expb" {
		t.Fatalf("experiments = %v", exps)
	}
	r := findRun(t, snap, "expa/runs/arm1")
	if r.Group != "expa/runs" || r.Name != "arm1" || r.Experiment != "expa" {
		t.Fatalf("grouping = %q %q %q", r.Experiment, r.Group, r.Name)
	}
	var arts []string
	for _, a := range snap.Artifacts {
		arts = append(arts, a.ID)
	}
	if strings.Join(arts, ",") != "expa/fit/report.md,expa/runs/summary.md" {
		t.Fatalf("artifacts = %v", arts)
	}
}

func TestParseTolerance(t *testing.T) {
	snap := scanAt(t, time.Minute)
	r := findRun(t, snap, "expa/runs/arm1")
	if r.KLTarget == nil || *r.KLTarget != 0.1 {
		t.Fatalf("kl target = %v", r.KLTarget)
	}
	if r.PlannedRounds == nil || *r.PlannedRounds != 3 {
		t.Fatalf("planned rounds = %v", r.PlannedRounds)
	}
	if r.Control == nil || *r.Control.WinRate != 0.51 || len(r.Control.Pairs) != 2 {
		t.Fatalf("control = %+v", r.Control)
	}
	// round3's eval.json is truncated: skipped with a warning, collect kept.
	var ns []int
	for _, rd := range r.Rounds {
		ns = append(ns, rd.N)
	}
	if len(ns) != 4 || ns[3] != 3 {
		t.Fatalf("rounds = %v", ns)
	}
	r3 := r.Rounds[3]
	if r3.Eval != nil || r3.Collect == nil || *r3.Collect.WinRate != 0.41 {
		t.Fatalf("round3 = %+v", r3)
	}
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "round3/eval.json") {
		t.Fatalf("warnings = %v", r.Warnings)
	}
	// wrong-typed fields are dropped, not fatal.
	r1 := r.Rounds[1]
	if r1.Eval.MeanTurns != nil || *r1.Eval.WinRate != 0.48 {
		t.Fatalf("round1 eval = %+v", r1.Eval)
	}
	r2 := r.Rounds[2].Stats
	if len(r2.Kinds) != 2 || r2.Kinds[1].FinalKL != nil || *r2.Kinds[0].FinalKL != 1.04 {
		t.Fatalf("round2 kinds = %+v", r2.Kinds)
	}
	if r.CurrentRound == nil || *r.CurrentRound != 3 || r.Stage != "[round 3] train ..." {
		t.Fatalf("stage = %v %q", r.CurrentRound, r.Stage)
	}
	if r.Timing == nil || len(r.Timing.Rows) != 3 || r.Timing.Header[2] != "seconds" {
		t.Fatalf("timing = %+v", r.Timing)
	}
	// arm2's round1/collect.json is zero bytes (a stage mid-redirect): absent,
	// not a warning.
	if a2 := findRun(t, snap, "expa/runs/arm2"); len(a2.Warnings) != 0 || a2.Rounds[0].Collect != nil {
		t.Fatalf("arm2 = %+v", a2)
	}
	// genN layout: the widest eval-<kinds>.json is used.
	g := findRun(t, snap, "expb/cmp/run3")
	if len(g.Rounds) != 1 || *g.Rounds[0].Eval.WinRate != 0.35 {
		t.Fatalf("gen run = %+v", g.Rounds)
	}
	if g.Status != "done" && g.Status != "running" {
		t.Fatalf("gen run status %q", g.Status)
	}
}

func TestLiveness(t *testing.T) {
	if r := findRun(t, scanAt(t, time.Minute), "expa/runs/arm1"); r.Status != "running" {
		t.Fatalf("fresh stderr: status %q, want running", r.Status)
	}
	if r := findRun(t, scanAt(t, 10*time.Minute), "expa/runs/arm1"); r.Status != "stale" {
		t.Fatalf("old stderr ending in '...': status %q, want stale", r.Status)
	}
	arm2, err := os.Stat(filepath.Join(fixtureRoot, "expa/runs/arm2.stderr"))
	if err != nil {
		t.Fatal(err)
	}
	s := NewScanner([]string{fixtureRoot})
	s.Now = func() time.Time { return arm2.ModTime().Add(10 * time.Minute) }
	if r := findRun(t, s.Scan(), "expa/runs/arm2"); r.Status != "done" {
		t.Fatalf("finished run: status %q, want done", r.Status)
	}
}

func TestAlerts(t *testing.T) {
	r := findRun(t, scanAt(t, time.Minute), "expa/runs/arm1")
	got := map[string]Alert{}
	for _, a := range r.Alerts {
		got[a.Rule] = a
	}
	if len(r.Alerts) != 3 {
		t.Fatalf("alerts = %+v, want below_control+drop+kl on r2", r.Alerts)
	}
	for _, rule := range []string{"below_control", "drop", "kl"} {
		a, ok := got[rule]
		if !ok || a.Round != 2 || !a.Latest || a.Run != "expa/runs/arm1" {
			t.Fatalf("%s alert = %+v", rule, a)
		}
	}
	if got["kl"].Kind != "attackers" {
		t.Fatalf("kl kind = %q", got["kl"].Kind)
	}
}

func TestAlertRuleEdges(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	kl := 0.1
	r := &Run{ID: "x", Status: "running", KLTarget: &kl, Control: &EvalResult{WinRate: f(0.5)},
		Rounds: []Round{
			{N: 0, Eval: &EvalResult{WinRate: f(0.46)}},                                                              // within margin
			{N: 1, Eval: &EvalResult{WinRate: f(0.40)}},                                                              // below control, drop 0.06
			{N: 2, Eval: &EvalResult{WinRate: f(0.50)}, Stats: &Stats{Kinds: []Kind{{Kind: "a", FinalKL: f(0.29)}}}}, // recovers, KL under 3x
			{N: 3, Eval: &EvalResult{WinRate: nil}},
		}}
	as := RunAlerts(r)
	if len(as) != 1 || as[0].Rule != "below_control" || as[0].Round != 1 || as[0].Latest {
		t.Fatalf("alerts = %+v", as)
	}
	r.KLTarget = nil
	r.Control = nil
	r.Rounds[2].Stats.Kinds[0].FinalKL = f(9)
	if as := RunAlerts(r); len(as) != 0 {
		t.Fatalf("no control/target: alerts = %+v", as)
	}
}

func TestHTTP(t *testing.T) {
	s := NewScanner([]string{fixtureRoot})
	srv := httptest.NewServer(newMux(s))
	defer srv.Close()

	res, err := srv.Client().Get(srv.URL + "/api/runs")
	if err != nil {
		t.Fatal(err)
	}
	var snap Snapshot
	if err := json.NewDecoder(res.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if len(snap.Experiments) != 2 || len(snap.Alerts) != 3 {
		t.Fatalf("api snapshot: %d experiments, %d alerts", len(snap.Experiments), len(snap.Alerts))
	}
	for path, want := range map[string]int{
		"/":                                    200,
		"/api/artifact?id=expa/fit/report.md":  200,
		"/api/artifact?id=../../../etc/passwd": 404,
		"/api/artifact?id=expa/runs/arm1/plan.txt": 404,
	} {
		res, err := srv.Client().Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != want {
			t.Fatalf("GET %s = %d, want %d", path, res.StatusCode, want)
		}
		if path == "/" && !strings.Contains(string(b), "<title>traindash</title>") {
			t.Fatalf("index page not served")
		}
	}
}

// TestReadOnly: a scan never creates or modifies anything under the root.
func TestReadOnly(t *testing.T) {
	before := treeState(t)
	s := NewScanner([]string{fixtureRoot})
	s.Scan()
	s.Scan()
	if after := treeState(t); after != before {
		t.Fatalf("scan changed the fixture tree")
	}
}

func treeState(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	filepath.Walk(fixtureRoot, func(p string, fi os.FileInfo, err error) error {
		if err == nil {
			b.WriteString(p + " " + fi.ModTime().String() + "\n")
		}
		return nil
	})
	return b.String()
}
