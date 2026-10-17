package azmcts

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestEncoderGolden is an equivalence harness for encoder and prior
// optimisations: it plays default-bot games over fixed deck pairs and, at
// every decision, digests the exact bits of every non-diagnostic feature
// set's state and option encodings and, with a network, the search's
// ranking and plain priors. ENCPERF_GOLDEN names the dump file (the test
// skips without it); ENCPERF_WRITE=1 writes it, otherwise the digests are
// compared line by line. ENCPERF_NET is a .dzgw network for the prior lines.
// The dump is generated from a baseline build and never committed.
func TestEncoderGolden(t *testing.T) {
	path := os.Getenv("ENCPERF_GOLDEN")
	if path == "" {
		t.Skip("ENCPERF_GOLDEN unset")
	}
	var net *policynet.Model
	if p := os.Getenv("ENCPERF_NET"); p != "" {
		m, err := gonet.Model(p)
		if err != nil {
			t.Fatal(err)
		}
		net = m
	}
	lines := encGoldenLines(t, net)
	if os.Getenv("ENCPERF_WRITE") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d lines to %s", len(lines), path)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	i := 0
	for sc.Scan() {
		if i >= len(lines) {
			t.Fatalf("golden has more lines than the run (%d)", len(lines))
		}
		if sc.Text() != lines[i] {
			t.Fatalf("line %d differs:\n golden %s\n got    %s", i+1, sc.Text(), lines[i])
		}
		i++
	}
	if i != len(lines) {
		t.Fatalf("run has %d lines, golden %d", len(lines), i)
	}
	t.Logf("%d lines identical", i)
}

var encGoldenPairs = [][2]string{
	{"uw-tempo", "mono-blue-tempo"},
	{"mono-red-goblins", "mono-green-stompy"},
	{"death-n-taxes", "ur-delver"},
	{"boros-moxite-burn", "dimir-tempo"},
	{"mono-white-equipment", "eldrazi-stompy"},
	{"tron", "uw-control"},
}

var encGoldenSets = []policynet.FeatureSet{policynet.FeaturesV1, policynet.FeaturesMZ, policynet.FeaturesEntity, policynet.FeaturesMZOwnLib, policynet.FeaturesMZGrave}

func encGoldenLines(t *testing.T, net *policynet.Model) []string {
	var out []string
	kinds := FullKinds()
	for gi, pr := range encGoldenPairs {
		cfg := testConfig(t, pr[0], pr[1], 4242+uint64(gi))
		e := rules.New(cfg)
		e.Advance()
		rngs := searchprobe.BotRandoms(cfg.Seed, 2)
		board := botpolicy.NewBoard(2)
		var pv view.View
		for step := 0; step < 400 && !e.G.Over; step++ {
			d := e.Pending()
			if d == nil {
				break
			}
			in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
			view.ProjectInto(&pv, e.G, e, d.Player, d)
			v := pv
			for _, fs := range encGoldenSets {
				st := policynet.EncodeStateWith(fs, v, d.Player, nil)
				enc := make([]policynet.Option, len(d.Options))
				for i := range d.Options {
					enc[i] = policynet.EncodeOptionWith(fs, v, d.Player, d.Kind, d.Options[i], i, len(d.Options))
				}
				out = append(out, fmt.Sprintf("g%d s%d p%d %s %s st=%s op=%s", gi, step, d.Player, d.Kind, fs, digestState(&st), digestOpts(enc)))
			}
			if net != nil {
				for _, auto := range []bool{false, true} {
					cands, kind, _, ok, _ := enumerateCut(searchprobe.NewCollector(d.Player), e, d, in, kinds, DefaultOptions().Limit, auto)
					if !ok || len(cands) < 2 {
						continue
					}
					var pv2 view.View
					p0, fell0 := priors(net, e, d, in, kind, cands, &pv2)
					kept, p1, fell1, before := rankedPrior(net, 6, 0, e, d, in, kind, cands, &pv2)
					keys := make([]string, len(kept))
					for i, c := range kept {
						keys[i] = fmt.Sprintf("%x", sha256.Sum256([]byte(c.key)))[:8]
					}
					out = append(out, fmt.Sprintf("g%d s%d p%d %s auto=%v n=%d prior=%s %v ranked=%s %v %d keys=%s", gi, step, d.Player, kind, auto, len(cands),
						f64bits(p0), fell0, f64bits(p1), fell1, before, strings.Join(keys, ",")))
				}
			}
			if err := e.Submit(in); err != nil {
				t.Fatalf("pair %d step %d: %v", gi, step, err)
			}
		}
	}
	return out
}

func f64bits(xs []float64) string {
	var b strings.Builder
	for i, x := range xs {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%016x", math.Float64bits(x))
	}
	return b.String()
}

func digestFeats(h []byte, fs []policynet.Feature) []byte {
	h = binary.LittleEndian.AppendUint32(h, uint32(len(fs)))
	for _, f := range fs {
		h = binary.LittleEndian.AppendUint16(h, f.Row)
		h = binary.LittleEndian.AppendUint32(h, math.Float32bits(f.Value))
	}
	return h
}

func digestF32(h []byte, xs []float32) []byte {
	h = binary.LittleEndian.AppendUint32(h, uint32(len(xs)))
	for _, x := range xs {
		h = binary.LittleEndian.AppendUint32(h, math.Float32bits(x))
	}
	return h
}

func digestState(st *policynet.State) string {
	var b []byte
	b = digestF32(b, st.Dense)
	b = digestFeats(b, st.Sparse)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(st.Cards)))
	for i := range st.Cards {
		c := &st.Cards[i]
		b = append(b, c.Group)
		b = digestF32(b, c.Raw)
		b = digestFeats(b, c.Rows)
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))[:16]
}

func digestOpts(opts []policynet.Option) string {
	parts := make([]string, len(opts))
	for i := range opts {
		o := &opts[i]
		var b []byte
		b = digestFeats(b, o.Slots)
		b = digestFeats(b, o.Hashed)
		b = digestF32(b, o.Dense)
		b = digestF32(b, o.Extra)
		b = binary.LittleEndian.AppendUint32(b, uint32(o.EntA))
		b = binary.LittleEndian.AppendUint32(b, uint32(o.EntB))
		if o.BotPick {
			b = append(b, 1)
		}
		parts[i] = fmt.Sprintf("%x", sha256.Sum256(b))[:12]
	}
	return strings.Join(parts, ",")
}

// BenchmarkPriorPath is the per-point prior cost the search pays (one op =
// one rankedPrior at topk=6, auto-pay candidates, over a fixed set of real
// default-bot positions; the network's result cache is warm after the first
// pass, so the op is dominated by the projection and the encoders).
func BenchmarkPriorPath(b *testing.B) {
	p := os.Getenv("ENCPERF_NET")
	if p == "" {
		b.Skip("ENCPERF_NET unset")
	}
	net, err := gonet.Model(p)
	if err != nil {
		b.Fatal(err)
	}
	type pos struct {
		e    *rules.Engine
		in   decision.Intent
		kind string
		c    []cand
	}
	var ps []pos
	kinds := FullKinds()
	for gi, pr := range encGoldenPairs[:3] {
		cfg := testConfig(b, pr[0], pr[1], 4242+uint64(gi))
		e := rules.New(cfg)
		e.Advance()
		rngs := searchprobe.BotRandoms(cfg.Seed, 2)
		board := botpolicy.NewBoard(2)
		for step := 0; step < 400 && !e.G.Over; step++ {
			d := e.Pending()
			if d == nil {
				break
			}
			in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
			if cands, kind, _, ok, _ := enumerateCut(searchprobe.NewCollector(d.Player), e, d, in, kinds, DefaultOptions().Limit, true); ok && len(cands) >= 2 {
				ps = append(ps, pos{e: e.Clone(), in: in, kind: kind, c: cands})
			}
			if err := e.Submit(in); err != nil {
				b.Fatal(err)
			}
		}
	}
	var pv view.View
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := &ps[i%len(ps)]
		rankedPrior(net, 6, 0, q.e, q.e.Pending(), q.in, q.kind, q.c, &pv)
	}
}
