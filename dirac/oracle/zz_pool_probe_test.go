package oracle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// TEMP probe: dump gorge's pool (and restricted batch) at the priority ask the
// recorded game diverges at. Delete before commit.
func TestZZPoolProbeHistorian(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "mana_paid_gap_cast_offer.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
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
	tbl, _, _, err := globSet(t, r.Pool, r).Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	rc.Finish = func(g *GorgeSide) {
		var b strings.Builder
		if d := g.Pending(); d != nil {
			fmt.Fprintf(&b, "FINAL pending kind=%v prompt=%q\n", d.Kind, d.Prompt)
			for _, o := range d.Options {
				fmt.Fprintf(&b, "  final opt idx=%d kind=%s label=%q obj=%d mode=%q alt=%d\n", o.Index, o.Kind, o.Label, o.Obj, o.Mode, o.AltCostIndex)
			}
		}
		for pl := state.PlayerID(0); int(pl) < len(g.E.G.Players); pl++ {
			p := g.E.G.Players[pl]
			fmt.Fprintf(&b, "PROBE pool p%d=%v restricted=%d batches:", pl, p.Pool, len(p.RestrictedMana))
			for i := range p.RestrictedMana {
				fmt.Fprintf(&b, " [%v]", p.RestrictedMana[i])
			}
			b.WriteString("\n")
		}
		f, _ := os.OpenFile("/home/sadams/projects/gorge/.worktrees/agent-20261121T132739Z-91fa87bd/.ds4/scratch/probe.out", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		f.WriteString(b.String())
		f.Close()
	}
	k, err := SetSide(globSet(t, r.Pool, r), tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	pol := Policy(&ScriptPolicy{Actions: r.Actions})
	pol = probePolicy{pol, 0, 1 << 30}
	res := Lockstep(m, r.Seed, k, pol, rc)
	if res.Div == nil {
		t.Fatal("no divergence (arm active?)")
	}
	t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}

// probePolicy dumps the gorge side at every decision in [lo,hi).
type probePolicy struct {
	p      Policy
	lo, hi int
}

func (w probePolicy) Pick(c *PickCtx) Canon {
	if c.Seq >= w.lo && c.Seq < w.hi && c.G != nil {
		var b strings.Builder
		fmt.Fprintf(&b, "ASK %d options=%v\n", c.Seq, c.D.Options)
		if d := c.G.Pending(); d != nil {
			fmt.Fprintf(&b, "  raw kind=%v prompt=%q\n", d.Kind, d.Prompt)
			for _, o := range d.Options {
				fmt.Fprintf(&b, "  raw opt idx=%d kind=%s label=%q obj=%d mode=%q alt=%d\n", o.Index, o.Kind, o.Label, o.Obj, o.Mode, o.AltCostIndex)
			}
		}
		if c.Seq >= 770 {
			for _, id := range c.G.E.G.Zone(state.ZHand, 0) {
				if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil {
					f := o.Face()
					fmt.Fprintf(&b, "  hand #%d %s cost=%v\n", id, f.Name, f.ManaCost)
				}
			}
			for _, id := range c.G.E.G.Zone(state.ZBattlefield, 0) {
				if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && !o.Tapped {
					fmt.Fprintf(&b, "  bf   #%d %s tapped=false\n", id, o.Face().Name)
				}
			}
		}
		for pl := state.PlayerID(0); int(pl) < len(c.G.E.G.Players); pl++ {
			p := c.G.E.G.Players[pl]
			fmt.Fprintf(&b, "  pool p%d=%v restricted=%d:", pl, p.Pool, len(p.RestrictedMana))
			for i := range p.RestrictedMana {
				fmt.Fprintf(&b, " [%v]", p.RestrictedMana[i])
			}
			fmt.Fprintf(&b, " hand=%d", len(c.G.E.G.Zone(state.ZHand, pl)))
			b.WriteString("\n")
		}
		f, _ := os.OpenFile("/home/sadams/projects/gorge/.worktrees/agent-20261121T132739Z-91fa87bd/.ds4/scratch/probe.out", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		f.WriteString(b.String())
		f.Close()
	}
	return w.p.Pick(c)
}
