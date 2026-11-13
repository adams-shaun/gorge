package dirac

import (
	"testing"
)

// A restricted one-colour source a Taps watcher defers (Hydraulic Helper beside
// Captain America, Living Legend) is a script gap only while it can be tapped:
// gorge's census leaves a summoning-sick, tapped or locked source out and its
// prefix search cannot activate it, so the offer must not count its {U}, and
// restrDeferScriptable must not stall on it either.
func restrTapsGapScen(t *testing.T) (*scen, Slot) {
	t.Helper()
	sc, id := synthStdMshScen(t, "Hydraulic Helper", "Captain America, Living Legend")
	sl := sc.slot(sc.perm(id["Hydraulic Helper"], 0))
	sc.perm(id["Captain America, Living Legend"], 0)
	return sc, sl
}

func TestRestrTapsGapNeedsAnActivatableSource(t *testing.T) {
	census := func(sc *scen) int {
		var a avail
		sc.s.availScript(sc.k, 0, &a)
		return a.total
	}
	t.Run("an untapped source is a counted script gap", func(t *testing.T) {
		sc, sl := restrTapsGapScen(t)
		if !sc.s.restrTapsGap(sc.k, 0, sl) {
			t.Fatal("precondition: the Helper is no Taps-deferred restricted gap beside Captain America")
		}
		if !sc.s.scriptTapsDefer(sc.k, 0).has(sl) {
			t.Fatal("the activatable Helper is not in the script mask")
		}
		if n := census(sc); n != 1 {
			t.Fatalf("census total %d, want 1 (the Helper's {U})", n)
		}
		if !sc.s.restrDeferScriptable(sc.k, 0, NoSlot) {
			t.Fatal("restrDeferScriptable is false for a scriptable gap")
		}
	})
	for name, set := range map[string]func(*scen, Slot){
		"summoning-sick": func(sc *scen, sl Slot) { sc.s.perms[sl].flags |= pfSick },
		"tapped":         func(sc *scen, sl Slot) { sc.s.perms[sl].flags |= pfTapped },
	} {
		t.Run(name+" source is no gap and counts nothing", func(t *testing.T) {
			sc, sl := restrTapsGapScen(t)
			set(sc, sl)
			if !sc.s.restrTapsGap(sc.k, 0, sl) {
				t.Fatal("precondition: the shape no longer reads as a restricted Taps gap")
			}
			if sc.s.tapsGapActivatable(sc.k, sl) {
				t.Fatal("precondition: the source is activatable")
			}
			if sc.s.scriptTapsDefer(sc.k, 0).has(sl) {
				t.Fatal("an unactivatable Helper is in the script mask")
			}
			if n := census(sc); n != 0 {
				t.Fatalf("census total %d, want 0", n)
			}
			if !sc.s.restrDeferScriptable(sc.k, 0, NoSlot) {
				t.Fatal("restrDeferScriptable stalls on a source that funds nothing")
			}
		})
	}
}
