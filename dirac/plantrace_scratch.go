package dirac

import (
	"fmt"
	"os"
)

// SCRATCH trace for the Vivid Crag payment hunt (ticket
// projection-vivid-crag-flags). NOT FOR COMMIT: delete before landing.

// DebugPlanTrace turns on the scratch planner trace below (set it from a
// scratch test before the lockstep run).
var DebugPlanTrace = os.Getenv("DIRAC_PLAN_TRACE") != ""

// DebugPlanTag labels the current run in the trace (SCRATCH).
var DebugPlanTag string

var debugPlanTrace = false

func colStr(m uint8) string {
	s := ""
	for c, ch := range "WUBRGC" {
		if m&(1<<uint(c)) != 0 {
			s += string(ch)
		}
	}
	return s
}

// traceTag appends msg to the scratch trace file (SCRATCH).
func traceTag(msg string) {
	f, err := os.OpenFile("/home/sadams/projects/gorge/.worktrees/agent-20261116T003840Z-b4605993/.ds4/scratch/plantrace.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	fmt.Fprintln(f, DebugPlanTag+" "+msg)
	f.Close()
}

// tracePlan prints the plan the kernel is about to tap (SCRATCH).
func (s *State) tracePlan(k *prep, p uint8, pk *planner) {
	f, err := os.OpenFile("/home/sadams/projects/gorge/.worktrees/agent-20261116T003840Z-b4605993/.ds4/scratch/plantrace.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, DebugPlanTag+" PLAN p=%d need=%v needAny=%d needT=%d nu=%d nt=%d found=%v anyColor=%v\n",
		p, pk.need, pk.needAny, pk.needT, pk.nu, pk.nt, pk.found, pk.anyColor)
	for t := 0; t < pk.nt; t++ {
		ty := &pk.types[t]
		fmt.Fprintf(f, "  type[%d] mask=%s dmask=%s n=%d creat=%v sac=%v sacOnly=%v avail=%d xv=%v\n",
			t, colStr(ty.mask), colStr(ty.dmask), ty.n, ty.creat, ty.sac, ty.sacOnly, ty.avail, ty.xv)
	}
	for u := 0; u < pk.nu; u++ {
		sl := pk.units[u]
		c := s.cardAt(sl)
		fmt.Fprintf(f, "  unit[%d] sl=%d obj=%d card=%q typ=%d rkind=%d\n",
			u, sl, pk.uobj[u], k.t.Cards[c].Name, pk.utyp[u], pk.rkind[u])
	}
	n := pk.bestN
	fmt.Fprintf(f, "  best (n=%d):", n)
	for i := 0; i < n; i++ {
		u := pk.bestSeq[i]
		sl := pk.units[u]
		c := s.cardAt(sl)
		fmt.Fprintf(f, " [%d %s %s col=%d]", sl, k.t.Cards[c].Name, colStr(pk.types[pk.bestTy[i]].mask), pk.bestCol[i])
	}
	fmt.Fprintln(f)
}
