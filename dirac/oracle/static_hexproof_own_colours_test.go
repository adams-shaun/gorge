package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestStaticHexproofOwnColoursRepro replays the recorded standard-set game:
// Tam, Mindful First-Year and a red creature on one side, an opponent's red
// damage spell on the other. The red creature is hexproof from red (its own
// colour) while Tam is not, so the spell can only be aimed at Tam. The test
// checks the game reached that shape: both creatures and the spell were cast,
// Tam took the damage, and the red creature took none.
func TestStaticHexproofOwnColoursRepro(t *testing.T) {
	const (
		tam  = "Tam, Mindful First-Year"
		red  = "Flamebraider"
		sear = "Sear"
	)
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", "static_hexproof_own_colours_red_spell.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set := reproSet(t, "std-ecl", r)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	casts := map[string]int{}
	for _, a := range r.Actions {
		if a.Kind() == ActCast {
			casts[m.InstanceCard(int(a.Src()))]++
		}
	}
	if casts[tam] == 0 || casts[red] == 0 || casts[sear] == 0 {
		t.Fatalf("precondition: the recorded game casts Tam %d, the red creature %d and the red spell %d, want one of each",
			casts[tam], casts[red], casts[sear])
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 50
	damage := map[string]int{}
	rc.Finish = func(g *GorgeSide) {
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind != events.Damage || e.Amount <= 0 {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
				damage[o.Face().Name] += int(e.Amount)
			}
		}
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	script := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, script, rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if damage[tam] == 0 {
		t.Fatal("the recorded game dealt no damage to Tam; the red spell never resolved")
	}
	if damage[red] != 0 {
		t.Fatalf("the red creature took %d damage though it is hexproof from red", damage[red])
	}
}
