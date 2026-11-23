package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// benchLine runs the command and parses its one stdout line.
func benchLine(t *testing.T, want int, args ...string) (m map[string]json.RawMessage) {
	t.Helper()
	var buf bytes.Buffer
	if got := run(args, &buf); got != want {
		t.Fatalf("run %v = %d, want %d; stdout %q", args, got, want, buf.String())
	}
	out := buf.String()
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("want exactly one line, got %q", out)
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not JSON: %v: %q", err, out)
	}
	return m
}

func intOf(t *testing.T, m map[string]json.RawMessage, k string) int {
	t.Helper()
	var n int
	if err := json.Unmarshal(m[k], &n); err != nil {
		t.Fatalf("%s: %v", k, err)
	}
	return n
}

func floatOf(t *testing.T, m map[string]json.RawMessage, k string) float64 {
	t.Helper()
	var f float64
	if err := json.Unmarshal(m[k], &f); err != nil {
		t.Fatalf("%s: %v", k, err)
	}
	return f
}

func requireCorpus(t *testing.T) {
	t.Helper()
	if _, err := cardset.CorpusDir(); errors.Is(err, cardset.ErrNoCorpus) {
		t.Skip(err)
	}
}

func TestShadowBenchVanillaSmoke(t *testing.T) {
	requireCorpus(t)
	m := benchLine(t, 0, "-pool", "vanilla", "-games", "4", "-sims", "16", "-worlds", "4")
	if n := intOf(t, m, "games"); n != 4 {
		t.Errorf("games %d, want 4", n)
	}
	if n := intOf(t, m, "refused"); n != 0 {
		t.Errorf("refused %d, want 0", n)
	}
	if n := intOf(t, m, "searched"); n <= 0 {
		t.Errorf("searched %d, want > 0", n)
	}
	wr, lo, hi := floatOf(t, m, "win_rate"), floatOf(t, m, "wilson_lo"), floatOf(t, m, "wilson_hi")
	if wr < lo || wr > hi {
		t.Errorf("win_rate %v outside [%v, %v]", wr, lo, hi)
	}
	// the same flags give the same line
	var a, b bytes.Buffer
	args := []string{"-games", "2", "-sims", "8", "-worlds", "4", "-seed", "7"}
	if run(args, &a) != 0 || run(args, &b) != 0 || a.String() != b.String() {
		t.Errorf("two runs differ:\n%s%s", a.String(), b.String())
	}
}

func TestShadowBenchSetCensus(t *testing.T) {
	cardsettest.Require(t, "fdn")
	m := benchLine(t, 0, "-pool", "set:fdn", "-games", "2", "-sims", "8")
	var reasons map[string]int
	if err := json.Unmarshal(m["reasons"], &reasons); err != nil || len(reasons) == 0 {
		t.Errorf("reasons %v (%v), want non-empty", reasons, err)
	}
	var refusals []struct {
		Detail string
		N      int
	}
	var raw [][2]json.RawMessage
	if err := json.Unmarshal(m["card_refusals"], &raw); err != nil {
		t.Fatal(err)
	}
	for _, r := range raw {
		var e struct {
			Detail string
			N      int
		}
		if json.Unmarshal(r[0], &e.Detail) != nil || json.Unmarshal(r[1], &e.N) != nil {
			t.Fatalf("bad card_refusals row %s", r)
		}
		refusals = append(refusals, e)
	}
	if len(refusals) > topCardRefusals {
		t.Errorf("%d card refusals, want at most %d", len(refusals), topCardRefusals)
	}
	if !sort.SliceIsSorted(refusals, func(i, j int) bool {
		a, b := refusals[i], refusals[j]
		return a.N > b.N || a.N == b.N && a.Detail < b.Detail
	}) {
		t.Errorf("card_refusals not sorted by count then text: %v", refusals)
	}
}

func TestShadowBenchFlags(t *testing.T) {
	for _, args := range [][]string{
		{"-pool", "nope"},
		{"-pool", "set:"},
		{"-games", "0"},
		{"-sims", "10", "-worlds", "4"},
		{"-worlds", "0"},
		{"stray"},
	} {
		var buf bytes.Buffer
		if got := run(args, &buf); got != 2 || buf.Len() != 0 {
			t.Errorf("run %v = %d with stdout %q, want exit 2 and none", args, got, buf.String())
		}
	}
}

func TestShadowBenchWilson(t *testing.T) {
	for _, c := range [][2]int{{0, 0}, {0, 4}, {4, 4}, {2, 4}, {37, 100}} {
		lo, hi := wilson(c[0], c[1])
		p := 0.0
		if c[1] > 0 {
			p = float64(c[0]) / float64(c[1])
		}
		if lo < 0 || hi > 1 || lo > p || p > hi {
			t.Errorf("wilson%v = [%v, %v], p %v", c, lo, hi, p)
		}
	}
	if lo, hi := wilson(50, 100); lo < 0.40 || lo > 0.41 || hi < 0.59 || hi > 0.60 {
		t.Errorf("wilson(50,100) = [%v, %v], want about [0.404, 0.596]", lo, hi)
	}
}
