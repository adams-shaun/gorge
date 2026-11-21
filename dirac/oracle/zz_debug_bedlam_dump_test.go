package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/state"
)

// Throwaway diagnostic: dump the kernel's repricing at kernel decision 453
// and gorge's grave at game end (the divergence).
func TestZZDebugBedlamDump(t *testing.T) {
	r, err := ReadRepro("testdata/repro/bedlam_reveler_reducecost_x.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	set := globSet(t, r.Pool, r)
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	rc.Finish = func(g *GorgeSide) {
		if d := g.E.Pending(); d != nil {
			fmt.Printf("== pending at end: kind=%v p%d options=%d\n", d.Kind, d.Player, len(d.Options))
		}
		for _, pp := range g.E.PotentialPaymentPlans(0) {
			kind, obj, ab := pp.Action.Kind, pp.Action.Obj, pp.Action.Ability
			nm := ""
			if o := g.E.G.Obj(obj); o != nil && o.Face() != nil {
				nm = o.Face().Name
			}
			fmt.Printf("== plan p0: kind=%q obj=%d(%s) ab=%v mode=%q reason=%q script=%d hasPlan=%v\n",
				kind, obj, nm, ab, pp.Action.Mode, pp.Reason, len(pp.Script), pp.Plan != nil)
			for si, st := range pp.Script {
				fmt.Printf("   script[%d] kind=%v picks:", si, st.Kind)
				for _, pk := range st.Picks {
					sn := ""
					if o := g.E.G.Obj(pk.Obj); o != nil && o.Face() != nil {
						sn = o.Face().Name
					}
					fmt.Printf(" (%s %s#%d ab=%d sym=%q)", pk.Kind, sn, pk.Obj, pk.Ability, pk.ManaSymbol)
				}
				fmt.Println()
			}
			if pp.Plan != nil {
				for _, act := range pp.Plan.Activations {
					src := g.E.G.Obj(act.Source)
					sn := ""
					if src != nil && src.Face() != nil {
						sn = src.Face().Name
					}
					fmt.Printf("   act src=%d(%s) ability=%v\n", act.Source, sn, act.Ability)
				}
			}
		}
		for pl := state.PlayerID(0); pl < 2; pl++ {
			fmt.Printf("== gorge grave p%d:", pl)
			for _, id := range g.E.G.Zone(state.ZGraveyard, pl) {
				o := g.E.G.Obj(id)
				nm := ""
				if o.Card != nil && len(o.Card.Faces) > 0 {
					nm = o.Card.Faces[0].Name
				}
				fmt.Printf(" %s(#%d owner=p%d)", nm, id, o.Owner)
			}
			fmt.Println()
		}
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	dirac.ZzSetTrace(true)
	defer dirac.ZzSetTrace(false)
	pol := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, pol, rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
}
