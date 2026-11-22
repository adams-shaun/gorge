package oracle

import (
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// zz_scratch_diag_test.go is a throwaway diagnostic: it replays the shrunken
// Iron Fist repro and dumps gorge's state at the divergence ask (pools,
// untapped permanents, hand). Delete before commit.
func TestZZScratchDiagDumpState(t *testing.T) {
	r, err := ReadRepro("testdata/repro/zz_scratch_diag.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDiagValue(t, r, pol)
	if res.Div != nil {
		t.Logf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
}

func replayDiagValue(t *testing.T, r *Repro, pol Policy) Result {
	t.Helper()
	reg := synthReg(t)
	m, err := r.Matchup(reg)
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
	rc.MaxDecisions = len(r.Actions) + 1
	ncheck := 0
	rc.Check = func(ref *GorgeSide, k Side) *Divergence {
		ncheck++
		if ncheck >= 635 {
			units := ref.E.WindowManaUnitsWith(0, nil, nil)
			f, ferr := os.OpenFile("../../.ds4/scratch/gorge.units", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
			if ferr != nil {
				t.Fatal(ferr)
			}
			for _, u := range units {
				fmt.Fprintf(f, "check#%d unit id=%d free=%d alts:", ncheck, u.ID, u.FreeCount)
				for _, al := range u.Alts {
					fmt.Fprintf(f, " [ma=%v counts=%v amt=%d any=%v freeTap=%v]", al.Ma != nil, al.Counts, al.Amt, al.Any, al.FreeTap)
				}
				fmt.Fprintln(f)
			}
			f.Close()
		}
		f, ferr := os.OpenFile("../../.ds4/scratch/diag.checks", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
		if ferr != nil {
			t.Fatal(ferr)
		}
		defer f.Close()
		pl := &ref.E.G.Players[0]
		fmt.Fprintf(f, "check#%d pool=%v life=%d step=%v active=p%d turn=%d hand:", ncheck, pl.Pool, pl.Life, ref.E.G.Step, ref.E.G.Active, ref.E.G.Turn)
		for _, id := range ref.E.G.Zone(state.ZHand, 0) {
			o := ref.E.G.Obj(id)
			fmt.Fprintf(f, " %s#%d", o.Card.Faces[0].Name, id)
		}
		fmt.Fprintf(f, " bf:")
		for _, id := range ref.E.G.Zone(state.ZBattlefield, 0) {
			o := ref.E.G.Obj(id)
			if o == nil || o.Card == nil {
				continue
			}
			fmt.Fprintf(f, " %s#%d(t=%v)", o.Card.Faces[0].Name, id, o.Tapped)
		}
		fmt.Fprintln(f)
		return nil
	}
	rc.Finish = func(g *GorgeSide) {
		f, ferr := os.Create("../../.ds4/scratch/diag.dump")
		if ferr != nil {
			t.Fatal(ferr)
		}
		defer f.Close()
		for pl := state.PlayerID(0); int(pl) < len(g.E.G.Players); pl++ {
			p := &g.E.G.Players[pl]
			fmt.Fprintf(f, "p%d pool=%v life=%d\n", pl, p.Pool, p.Life)
			fmt.Fprintf(f, "p%d hand:", pl)
			for _, id := range g.E.G.Zone(state.ZHand, pl) {
				o := g.E.G.Obj(id)
				fmt.Fprintf(f, " %s#%d(mc=%s)", o.Card.Faces[0].Name, id, o.Card.Faces[0].ManaCost)
			}
			fmt.Fprintln(f)
			fmt.Fprintf(f, "p%d bf:", pl)
			for _, id := range g.E.G.Zone(state.ZBattlefield, pl) {
				o := g.E.G.Obj(id)
				if o == nil || o.Card == nil {
					continue
				}
				fmt.Fprintf(f, " %s#%d(tapped=%v,sick=%v)", o.Card.Faces[0].Name, id, o.Tapped, o.SummonSick)
			}
			fmt.Fprintln(f)
		}
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc)
}
