package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// synthetic60 is 100 pods of 10 a-vs-b games, seats alternating, where a wins
// 4 to 8 of a pod's games (mean 6): a 60/40 split with pod-to-pod spread.
func synthetic60() []gameResult {
	var gs []gameResult
	for p := 0; p < 100; p++ {
		aWins := 6 + p%5 - 2
		for g := 0; g < 10; g++ {
			r := gameResult{Pod: p, A: "a", B: "b"}
			if g%2 == 1 {
				r.A, r.B = "b", "a"
			}
			if (g < aWins) != (r.A == "a") {
				r.Winner = 1
			}
			gs = append(gs, r)
		}
	}
	return gs
}

func TestBootstrapCIDeterministic(t *testing.T) {
	gs := synthetic60()
	first := bootstrap(gs, 7, bootstrapResamples)
	second := bootstrap(gs, 7, bootstrapResamples)
	if !slices.Equal(first, second) {
		t.Fatalf("same seed, different intervals:\n%v\n%v", first, second)
	}
	if len(first) != 2 || first[0].Label != "a" || first[1].Label != "b" {
		t.Fatalf("want labels a then b, got %v", first)
	}
	a := first[0]
	if a.Rate != 0.6 || a.Games != 1000 {
		t.Fatalf("a: want rate 0.6 over 1000 games, got %v over %d", a.Rate, a.Games)
	}
	if !(a.Lo < a.Hi) {
		t.Fatalf("a: degenerate interval [%v, %v]: the input has no spread to resample", a.Lo, a.Hi)
	}
	if !(a.Lo < 0.6 && 0.6 < a.Hi) {
		t.Fatalf("a: interval [%v, %v] misses 0.6", a.Lo, a.Hi)
	}
	if b := first[1]; !(b.Lo < 0.4 && 0.4 < b.Hi) || b.Rate != 0.4 {
		t.Fatalf("b: want 0.4 inside its interval, got %v [%v, %v]", b.Rate, b.Lo, b.Hi)
	}
}

func TestChunkResumeSkipsDone(t *testing.T) {
	out := t.TempDir()
	plans := planChunks(10, 4)
	if len(plans) != 3 || plans[2] != (chunkPlan{Index: 2, First: 8, Last: 10}) {
		t.Fatalf("10 pods in chunks of 4: got %v", plans)
	}
	d0 := chunkDir(out, 0)
	if err := os.MkdirAll(d0, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d0, doneMarker), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// A partial chunk 1 from an interrupted run: redone from scratch.
	d1 := chunkDir(out, 1)
	if err := os.MkdirAll(d1, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(d1, "pairs.tsv")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !chunkDone(d0) || chunkDone(d1) {
		t.Fatalf("precondition: chunk 0 done, chunk 1 not")
	}
	var called []chunkPlan
	ran, err := runChunks(out, plans, func(c chunkPlan, dir string) error {
		if dir != chunkDir(out, c.Index) {
			t.Errorf("chunk %d ran in %s", c.Index, dir)
		}
		called = append(called, c)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ran, []int{1, 2}) || len(called) != 2 || called[0] != plans[1] {
		t.Fatalf("want chunks 1 and 2 run, got ran=%v called=%v", ran, called)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("partial chunk 1's file survived the rerun: %v", err)
	}
	ran, err = runChunks(out, plans, func(c chunkPlan, dir string) error {
		t.Errorf("chunk %d ran on a finished run", c.Index)
		return nil
	})
	if err != nil || len(ran) != 0 {
		t.Fatalf("second rerun: ran=%v err=%v", ran, err)
	}
}

func TestVariantSeatRotation(t *testing.T) {
	vs := []string{"a", "b", "c"}
	seen := map[[2]int]map[string]bool{}
	for pod := 0; pod < 6; pod++ {
		for seat := 0; seat < 8; seat++ {
			got := seatVariant(vs, pod, seat)
			if want := vs[(seat+pod)%3]; got != want {
				t.Fatalf("pod %d seat %d: got %s, want %s", pod, seat, got, want)
			}
			k := [2]int{pod / 3, seat}
			if seen[k] == nil {
				seen[k] = map[string]bool{}
			}
			seen[k][got] = true
		}
	}
	if seatVariant(vs, 1, 0) != "b" || seatVariant(vs, 2, 7) != "a" {
		t.Fatalf("spot check: pod 1 seat 0 = %s, pod 2 seat 7 = %s", seatVariant(vs, 1, 0), seatVariant(vs, 2, 7))
	}
	// Every seat sees every variant once per K pods.
	for k, s := range seen {
		if len(s) != 3 {
			t.Fatalf("pods %d-%d seat %d saw %v, want all 3 variants", 3*k[0], 3*k[0]+2, k[1], s)
		}
	}
}
