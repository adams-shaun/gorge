package host

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// faultySeat wraps a bot and misbehaves at decision n: it panics, or
// returns an intent the engine must reject.
type faultySeat struct {
	inner seat.Seat
	at    int
	n     *int
	mode  string
}

func (f *faultySeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	*f.n++
	if *f.n == f.at {
		switch f.mode {
		case "panic":
			panic("seat exploded on purpose")
		case "reject":
			return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{len(d.Options) + 5}}, nil
		}
	}
	return f.inner.Decide(ctx, v, d)
}

func faultyOptions(t *testing.T, dir, mode string) Options {
	o := testOptions(t)
	o.Dir = dir
	o.Seats = func(names []string, seed uint64) []seat.Seat {
		n := 0
		out := make([]seat.Seat, len(names))
		for i := range names {
			out[i] = &faultySeat{inner: seat.NewBot(seed ^ uint64(i+1)), at: 25, n: &n, mode: mode}
		}
		return out
	}
	return o
}

func TestAPanickingSeatCrashesTheMatchAndHaltsTheTable(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(faultyOptions(t, dir, "panic"))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", true)) // perpetual: must NOT restart
	s := r.OpenSession()
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	_ = r.Start("t1")
	r.Wait("t1")

	tab := r.Tables()[0]
	if tab.State != protocol.TableHalted || tab.Match != 1 {
		t.Fatalf("table %+v", tab)
	}
	ms, _ := r.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchCrashed {
		t.Fatalf("matches %+v", ms)
	}
	report, err := os.ReadFile(filepath.Join(dir, "crash", "t1-1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"seat exploded on purpose", "head:", "seq:", "goroutine"} {
		if !strings.Contains(string(report), want) {
			t.Fatalf("crash report lacks %q:\n%s", want, report)
		}
	}
	sc, _ := readSidecar(dir, "t1", 1)
	if sc.State != protocol.MatchCrashed || !strings.Contains(sc.Reason, "panic") {
		t.Fatalf("sidecar %+v", sc)
	}
	var halted, ended bool
	for _, f := range drainNow(s) {
		switch f.T {
		case protocol.TTableHalted:
			halted = true
		case protocol.TMatchEnd:
			ended = true
		}
	}
	if !halted || !ended {
		t.Fatalf("frames: halted=%v match_end=%v", halted, ended)
	}
}

func TestARejectedIntentCrashesTheMatch(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(faultyOptions(t, dir, "reject"))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.Start("t1")
	r.Wait("t1")
	ms, _ := r.Matches("t1")
	if ms[0].State != protocol.MatchCrashed {
		t.Fatalf("%+v", ms)
	}
	sc, _ := readSidecar(dir, "t1", 1)
	if !strings.Contains(sc.Reason, "rejected") || !strings.Contains(sc.Reason, "out of range") {
		t.Fatalf("reason %q", sc.Reason)
	}
	if r.Tables()[0].State != protocol.TableHalted {
		t.Fatal("table not halted")
	}
	if _, err := os.Stat(filepath.Join(dir, "crash", "t1-1.txt")); err != nil {
		t.Fatal("no crash report for a rejected intent")
	}
}

func TestANonTerminatingMatchIsACrash(t *testing.T) {
	o := testOptions(t)
	o.MaxIntents = 50
	r, _ := New(o)
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.Start("t1")
	r.Wait("t1")
	ms, _ := r.Matches("t1")
	if ms[0].State != protocol.MatchCrashed || r.Tables()[0].State != protocol.TableHalted {
		t.Fatalf("%+v / %+v", ms, r.Tables())
	}
}

func TestACrashedMatchStillReplaysFromDisk(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(faultyOptions(t, dir, "panic"))
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.Start("t1")
	r.Wait("t1")
	r.Close()
	r2, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	ms, _ := r2.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchCrashed {
		t.Fatalf("%+v", ms)
	}
	if _, err := r2.ViewAt("t1", 1, uint64(ms[0].Events-1)); err != nil {
		t.Fatalf("crashed match does not replay: %v", err)
	}
}
