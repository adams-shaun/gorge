package dirac

import "testing"

// synthMoleManScen builds p0's board from names and puts Mole Man, Moloid
// Master in p0's hand. Mole Man costs {2}{G}; none of the lands listed makes
// green on its own, so the cast is offered only when the combination filter
// (Baxter Building's {4}, {T}: four mana in any combination) is searched.
func synthMoleManScen(t *testing.T, names ...string) (*scen, Obj) {
	t.Helper()
	all := append([]string{"Mole Man, Moloid Master"}, names...)
	sc, id := synthScen(t, "std-msh", all...)
	for _, n := range names {
		sc.perm(id[n], 0)
	}
	return sc, sc.hand(id["Mole Man, Moloid Master"], 0)
}

// TestBaxterFilterPaysGreenBesideRestrictedSource is the std-msh lockstep
// divergence this change fixes: p1's board holds Baxter Building's combination
// filter beside Castle Doom's artifact-only any-colour ability and Hell's
// Kitchen's {B}/{R}. Gorge's prefix search offers Mole Man's {2}{G} through
// the filter; the kernel must too (CR 601.2g-h, 605.3a).
func TestBaxterFilterPaysGreenBesideRestrictedSource(t *testing.T) {
	t.Run("filterAlone", func(t *testing.T) {
		sc, mole := synthMoleManScen(t, "Swamp", "Swamp", "Swamp", "Swamp", "Baxter Building")
		f := sc.s.scriptFilt(sc.k, 0)
		var gaps Decision
		sc.s.plainGapsOptions(sc.k, &gaps, f)
		for i := 0; i < int(gaps.N); i++ {
			a := gaps.Options[i]
			t.Logf("gaps opt %d kind=%d src=%d mode=%d script=%v", i, a.Kind(), a.Source(), a.Mode(), sc.s.filtScript(sc.k, a, 0, false))
		}
		t.Logf("mole obj=%d code=%d", mole, TCode(uint8(mole)))
		if acts := sc.castOptions(mole); len(acts) == 0 {
			t.Fatal("Mole Man is not offered through Baxter Building's combination filter")
		}
	})
	t.Run("besideRestricted", func(t *testing.T) {
		sc, mole := synthMoleManScen(t, "Swamp", "Swamp", "Swamp", "Swamp", "Baxter Building", "Castle Doom", "Hell's Kitchen")
		f := sc.s.scriptFilt(sc.k, 0)
		t.Logf("filt=%v filtOnly=%v mix=%v gapOthers=%v anyR=%v restr=%v restrSrc=%v widening=%d funding=%d",
			f, sc.s.filtOnly(sc.k, 0), sc.s.mixBoard(sc.k, 0), sc.s.gapOthers(sc.k, 0), sc.s.scriptAnyR(sc.k, 0),
			sc.s.scriptRestr(sc.k, 0), sc.k.restrSrc, sc.k.gen.restrF, sc.s.filtFunding(sc.k, 0, f))
		var gaps Decision
		sc.s.plainGapsOptions(sc.k, &gaps, f)
		for i := 0; i < int(gaps.N); i++ {
			a := gaps.Options[i]
			t.Logf("gaps opt %d kind=%d src=%d mode=%d script=%v", i, a.Kind(), a.Source(), a.Mode(), sc.s.filtScript(sc.k, a, 0, false))
		}
		t.Logf("mole obj=%d code=%d", mole, TCode(uint8(mole)))
		if acts := sc.castOptions(mole); len(acts) == 0 {
			t.Fatal("Mole Man is not offered through Baxter Building beside Castle Doom's restricted source")
		}
	})
}
