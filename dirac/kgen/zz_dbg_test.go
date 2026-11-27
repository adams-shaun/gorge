package kgen

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestZZDbg(t *testing.T) {
	var sh miscShape
	for _, c := range miscShapes() {
		if c.card == "C.A.M.P." && c.start == nil {
			sh = c
		}
	}
	cardsDir, _ := cardset.CorpusDir()
	reg, _ := cards.SharedCorpus(cardsDir)
	p, _ := loadCardNames(miscCardNames(sh.matchups), cardsDir)
	pd, _, _, err := tableOf(p, "miscshapes-dbg", nil)
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	tbl, _, _, _ := set.Matchup(sh.matchups[0][0], sh.matchups[0][1])
	var gd [2][]*cards.Card
	for s, d := range sh.matchups[0] {
		for _, e := range d {
			c, _ := reg.Lookup(e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	seed := uint64(8)
	m, err := oracle.NewMatchup(reg, [2]string{"misc-a", "misc-b"}, gd, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg := oracle.Config{GorgeAutopaySurface: seed%4 == 0}
	k, err := oracle.SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := oracle.RandomPolicy(seed)
	out, _ := os.Create("/home/sadams/projects/gorge/.worktrees/agent-20261127T092044Z-64894bdb/.ds4/scratch/dbgout.txt")
	defer out.Close()
	var buf strings.Builder
	var lastRef *oracle.GorgeSide
	n := 0
	seen := 0
	res := oracle.Lockstep(m, seed, k, pol, oracle.RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: 40,
		Check: func(ref *oracle.GorgeSide, _ oracle.Side) *oracle.Divergence {
			n++
			lastRef = ref
			evs := ref.E.L.Events
			if true {
				fmt.Fprintf(&buf, "--- check %d (%d events)\n", n, len(evs))
				for _, ev := range evs[seen:] {
					name := ""
					if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil {
						name = o.Face().Name
					}
					fmt.Fprintf(&buf, "  %v obj=%d(%s) p=%d amt=%d ctr=%q txt=%q\n", ev.Kind, ev.Obj, name, ev.Player, ev.Amount, ev.Counter, ev.Text)
				}
			}
			seen = len(evs)
			return nil
		}})
	var pg, pk oracle.Projection
	lastRef.Project(&pg)
	k.Project(&pk)
	fmt.Fprintf(&buf, "--- check END\nPOOL ref=%v ker=%v NStack ref=%d ker=%d\n", pg.Pool[0], pk.Pool[0], pg.NStack, pk.NStack)
	for id := state.ObjID(1); id < 90; id++ {
		if o := lastRef.E.G.Obj(id); o != nil && o.Zone == state.ZBattlefield && o.Face() != nil {
			fmt.Fprintf(&buf, "bf obj=%d %s tapped=%v attachedTo=%d\n", id, o.Face().Name, o.Tapped, o.AttachedTo)
		}
	}
	for i := 0; i < int(pg.NInst) && i < len(pg.Cards); i++ {
		if pg.Cards[i].Flags != pk.Cards[i].Flags || pg.Cards[i].Zone != pk.Cards[i].Zone {
			fmt.Fprintf(&buf, "card %d zone %d/%d flags %d/%d\n", i, pg.Cards[i].Zone, pk.Cards[i].Zone, pg.Cards[i].Flags, pk.Cards[i].Flags)
		}
	}
	for _, ev := range lastRef.E.L.Events[seen:] {
		name := ""
		if o := lastRef.E.G.Obj(ev.Obj); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		fmt.Fprintf(&buf, "  %v obj=%d(%s) p=%d amt=%d ctr=%q txt=%q\n", ev.Kind, ev.Obj, name, ev.Player, ev.Amount, ev.Counter, ev.Text)
	}
	all := strings.Split(buf.String(), "--- check")
	if len(all) > 8 {
		all = all[len(all)-8:]
	}
	fmt.Fprintf(out, "--- check%s\ndiv=%+v\n", strings.Join(all, "--- check"), res.Div)
	t.Fatal("dbg done")
}
