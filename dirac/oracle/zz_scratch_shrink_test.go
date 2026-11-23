package oracle

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const zzPass Canon = 16776960

func TestZZScratchShrink(t *testing.T) {
	cfg := map[string]string{}
	b, err := os.ReadFile("../../.ds4/scratch/shrink.cfg")
	if err != nil {
		t.Skip()
	}
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if k, v, ok := strings.Cut(ln, "="); ok {
			cfg[k] = v
		}
	}
	out := cfg["OUT"]
	secs, _ := strconv.Atoi(cfg["SECS"])
	logf := func(f string, a ...any) {
		fh, err := os.OpenFile(out+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprintf(fh, f, a...)
			fh.Close()
		}
	}
	r, err := ReadRepro(cfg["IN"])
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := globSet(t, r.Pool, r)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	es := func(names []string) []cardset.Entry {
		o := make([]cardset.Entry, len(names))
		for i, n := range names {
			o[i] = cardset.Entry{Name: n, Count: 1}
		}
		return o
	}
	tbl, _, _, err := set.Matchup(es(r.Cards[0]), es(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	run := func(acts []Canon) (Result, int) {
		rr := rc
		rr.MaxDecisions = len(acts) + 200
		k, err := SetSide(set, tbl, rr.Config)(m, r.Seed)
		if err != nil {
			t.Error(err)
			return Result{}, 1
		}
		defer CloseSide(k)
		pol := &ScriptPolicy{Actions: acts}
		res := Lockstep(m, r.Seed, k, pol, rr)
		return res, pol.Skipped
	}
	cur := append([]Canon(nil), r.Actions...)
	base, sk := run(cur)
	if base.Div == nil || sk != 0 {
		t.Fatalf("base: div=%v skipped=%d", base.Div, sk)
	}
	wantAt := base.Div.At
	logf("BASE at %d ref=%v ker=%v n=%d\n", wantAt, base.Div.RefOptions, base.Div.KerOptions, len(cur))
	type outcome struct{ ok bool }
	same := func(d *Divergence) bool {
		return d != nil && d.At == wantAt && fmt.Sprint(d.RefOptions) == fmt.Sprint(base.Div.RefOptions) && fmt.Sprint(d.KerOptions) == fmt.Sprint(base.Div.KerOptions)
	}
	startChunk, startI := 64, 0
	if b, err := os.ReadFile(out + ".state"); err == nil {
		if strings.TrimSpace(string(b)) == "DONE" {
			return
		}
		fmt.Sscanf(string(b), "%d %d", &startChunk, &startI)
	}
	save := func(c, i int) { os.WriteFile(out+".state", []byte(fmt.Sprintf("%d %d", c, i)), 0o644) }
	deadline := time.Now().Add(time.Duration(secs) * time.Second)
	nonpass := func() []int {
		var ix []int
		for i, a := range cur {
			if a != zzPass {
				ix = append(ix, i)
			}
		}
		return ix
	}
	const par = 4
	for chunk := startChunk; chunk >= 1; chunk /= 2 {
		for i := startI; ; {
			np := nonpass()
			if i >= len(np) {
				break
			}
			if time.Now().After(deadline) {
				save(chunk, i)
				return
			}
			var cands [][]Canon
			var starts []int
			for j := 0; j < par && i+j*chunk < len(np); j++ {
				c := append([]Canon(nil), cur...)
				for _, p := range np[i+j*chunk : min(len(np), i+(j+1)*chunk)] {
					c[p] = zzPass
				}
				cands = append(cands, c)
				starts = append(starts, i+j*chunk)
			}
			oks := make([]bool, len(cands))
			var wg sync.WaitGroup
			for j := range cands {
				wg.Add(1)
				go func(j int) {
					defer wg.Done()
					res, sk := run(cands[j])
					oks[j] = sk == 0 && same(res.Div)
				}(j)
			}
			wg.Wait()
			won := -1
			for j, ok := range oks {
				if ok {
					won = j
					break
				}
			}
			if won < 0 {
				i += len(cands) * chunk
				continue
			}
			cur = cands[won]
			i = starts[won] // those slots are now pass: np shrinks, next group starts here
			nr := *r
			nr.Divergence = base.Div
			nr.setActions(cur)
			nr.Write(out)
			logf("chunk %d ok at np-index %d: %d non-pass left\n", chunk, starts[won], len(nonpass()))
		}
		startI = 0
		save(chunk/2, 0)
		logf("chunk %d done, %d non-pass\n", chunk, len(nonpass()))
	}
	os.WriteFile(out+".state", []byte("DONE"), 0o644)
}
