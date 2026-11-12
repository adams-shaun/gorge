package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func repeatName(name string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = name
	}
	return out
}

// TestOrcishBowmastersFirstCardInDrawStepLockstep plays a game where Orcish
// Bowmasters (or its rebalanced A- version) meets Howling Mine's extra draw in
// the opponent's draw step: gorge's FirstCardInDrawStep$ False trigger fires
// on the extra card and not on the turn-based one, and dirac must make the
// same decisions. Without the kernel's draw-step gate dirac fires on the
// turn-based draw too, so each game diverges early.
func TestOrcishBowmastersFirstCardInDrawStepLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range []struct{ name string }{
		{"Orcish Bowmasters"},
		{"A-Orcish Bowmasters"},
	} {
		t.Run(c.name, func(t *testing.T) {
			decks := [2][]string{
				append(append(repeatName(c.name, 4), repeatName("Howling Mine", 12)...), repeatName("Swamp", 20)...),
				repeatName("Plains", 40),
			}
			r := &Repro{Format: reproFormat, Decks: [2]string{"drawstep_first_0", "drawstep_first_1"},
				Cards: decks, Seed: 1, PassElision: true, Surface: "autopay", Kernel: "custom", MaxTurns: 20}
			set := printedDelveSet(t, r.Cards)
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			tbl, _, _, err := set.Matchup(oneOfEachEntries(decks[0]), oneOfEachEntries(decks[1]))
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= 4; seed++ {
				r.Seed = seed
				rc := r.RunConfig()
				rc.MaxDecisions = 1200
				var armies int
				rc.Finish = func(g *GorgeSide) {
					for p := state.PlayerID(0); p < 2; p++ {
						for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
							o := g.E.G.Obj(id)
							if o == nil || !o.IsToken || o.Face() == nil {
								continue
							}
							for _, ty := range o.Face().Types {
								if ty == "Army" {
									armies++
								}
							}
						}
					}
				}
				k, err := SetSide(set, tbl, rc.Config)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, seed, k, HeuristicPolicy(seed), rc)
				CloseSide(k)
				t.Logf("seed %d: %d decisions, %d turns, %d Army tokens, stall %q", seed, res.Decisions, res.Turns, armies, res.Stall)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !res.Clean() {
					t.Fatalf("seed %d: unclean game (stall %q)", seed, res.Stall)
				}
				if armies == 0 {
					t.Fatalf("seed %d: no Orc Army token: %s never resolved", seed, c.name)
				}
			}
		})
	}
}
