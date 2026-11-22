package oracle

import (
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Throwaway diagnostic. Delete before commit.
func TestTmpDiagBase(t *testing.T) {
	r, err := ReadRepro("testdata/repro/sac_other_mana_source.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	set := globSet(t, r.Pool, r)
	reg := synthReg(t)
	var n int
	var lf *os.File
	check := func(ref *GorgeSide, _ Side) *Divergence {
		n++
		if lf == nil {
			os.Remove("/tmp/probe.txt")
			lf, _ = os.Create("/tmp/probe.txt")
		}
		e := ref.E
		d := e.Pending()
		fmt.Fprintf(lf, "n=%d pending=%v kind=%v player=%v\n", n, d != nil, d != nil && d.Kind != "", d != nil && d.Acting() != 0 || d != nil && d.Acting() == 0)
		if n < 934 {
			return nil
		}
		if d == nil || d.Kind != decision.KPriority {
			return nil
		}
		fmt.Fprintf(lf, "pool=%v potentialMana=%v\n", e.G.Players[1].Pool, e.PotentialMana(1))
		pp := e.PotentialPaymentPlans(1)
		for i := range pp {
			p := &pp[i]
			scr := "nil"
			if p.Script != nil {
				scr = fmt.Sprintf("%d steps", len(p.Script))
			}
			fmt.Fprintf(lf, "pp %d: kind=%s obj=%d ability=%d mode=%q label=%q reason=%q detail=%q script=%s\n",
				i, p.Action.Kind, p.Action.Obj, p.Action.Ability, p.Action.Mode, p.Action.Label, p.Reason, p.Detail, scr)
		}
		var tram state.ObjID
		for _, id := range e.G.Zone(state.ZBattlefield, 1) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Tramway Station" {
				tram = id
			}
		}
		fmt.Fprintf(lf, "tramway=%d\n", tram)
		if tram != 0 {
			for _, b := range []int{200, 1000, 5000} {
				scr, reason := e.PotentialPlayScript(1, decision.PotentialAction{Kind: "ability", Obj: tram, Ability: 1}, b)
				fmt.Fprintf(lf, "PotentialPlayScript b=%d: script=%d steps reason=%q\n", b, len(scr), reason)
				if len(scr) > 0 {
					for i, st := range scr {
						fmt.Fprintf(lf, "  step %d: kind=%v picks=%d\n", i, st.Kind, len(st.Picks))
					}
				}
			}
		}
		return nil
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, nm := range names {
			es[i] = cardset.Entry{Name: nm, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	rc.Check = check
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div != nil {
		t.Fatalf("div at %d what=%s field=%s | ref %v | ker %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions)
	}
	t.Fatalf("clean, decisions=%d", res.Decisions)
}
